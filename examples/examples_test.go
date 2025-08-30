package examples

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestExamplesCompile tests that all examples compile successfully
func TestExamplesCompile(t *testing.T) {
	examples := []string{
		"./s3-usage",
		"./minio-demo",
	}

	for _, example := range examples {
		t.Run(example, func(t *testing.T) {
			cmd := exec.Command("go", "build", "-o", "/dev/null", example)
			cmd.Dir = "."
			
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("Failed to compile %s: %v\nOutput: %s", example, err, string(output))
			}
		})
	}
}

// TestExamplesRunWithoutPanic tests that examples can start without panicking
func TestExamplesRunWithoutPanic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping example run test in short mode")
	}

	examples := []struct {
		name string
		path string
	}{
		{"s3-usage", "./s3-usage"},
		{"minio-demo", "./minio-demo"},
	}

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			cmd := exec.Command("go", "run", example.path)
			cmd.Dir = "."
			
			// Set a timeout to prevent hanging
			done := make(chan error, 1)
			go func() {
				_, err := cmd.CombinedOutput()
				done <- err
			}()

			select {
			case err := <-done:
				// We expect this to fail with connection errors, but not panic
				if err != nil {
					// Check if it's a connection error (expected) vs a panic/compile error (unexpected)
					if strings.Contains(err.Error(), "exit status 1") {
						t.Logf("Example %s failed as expected (likely connection error)", example.name)
					} else {
						t.Errorf("Example %s failed unexpectedly: %v", example.name, err)
					}
				} else {
					t.Logf("Example %s completed successfully", example.name)
				}
			case <-time.After(10 * time.Second):
				// Kill the process if it's hanging
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
				t.Errorf("Example %s timed out (likely hanging)", example.name)
			}
		})
	}
}

// TestExamplesDocumentation tests that the examples have proper documentation
func TestExamplesDocumentation(t *testing.T) {
	// Check that README.md exists and has content
	readmePath := "README.md"
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("examples/README.md does not exist")
		return
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("Failed to read examples/README.md: %v", err)
	}

	readme := string(content)
	
	// Check for key sections
	requiredSections := []string{
		"S3 使用示例",
		"MinIO 头部演示", 
		"前置条件",
		"配置",
	}

	for _, section := range requiredSections {
		if !strings.Contains(readme, section) {
			t.Errorf("README.md missing required section: %s", section)
		}
	}

	// Check that it mentions the justfile commands
	justCommands := []string{
		"just run-s3-example",
		"just run-minio-example",
	}

	for _, cmd := range justCommands {
		if !strings.Contains(readme, cmd) {
			t.Errorf("README.md missing justfile command: %s", cmd)
		}
	}
}

// TestExamplesStructure tests that examples have the expected file structure
func TestExamplesStructure(t *testing.T) {
	expectedFiles := map[string][]string{
		"s3-usage": {
			"main.go",
			"main_test.go",
		},
		"minio-demo": {
			"main.go", 
			"main_test.go",
		},
	}

	for dir, files := range expectedFiles {
		t.Run(dir, func(t *testing.T) {
			for _, file := range files {
				path := dir + "/" + file
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("Expected file %s does not exist", path)
				}
			}
		})
	}
}