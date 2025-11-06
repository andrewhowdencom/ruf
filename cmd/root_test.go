package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// setupTestConfig creates a temporary directory and a dummy config file inside it.
// It returns the path to the directory, the path to the config file, and a cleanup function.
func setupTestConfig(t *testing.T, content string) (string, string, func()) {
	t.Helper()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}
	cleanup := func() {
		// viper.Reset() is called in each test's cleanup
	}
	return tempDir, configPath, cleanup
}

func TestInitConfig(t *testing.T) {
	// Cleanup viper after each test
	t.Cleanup(viper.Reset)

	t.Run("uses config from --config flag if set", func(t *testing.T) {
		t.Cleanup(viper.Reset)
		_, configPath, cleanup := setupTestConfig(t, "log:\n  level: debug")
		defer cleanup()

		// Simulate setting the --config flag
		cfgFile = configPath

		InitConfig()

		assert.Equal(t, configPath, viper.ConfigFileUsed())
		assert.Equal(t, "debug", viper.GetString("log.level"))
	})

	t.Run("uses /etc/ruf/config.yaml if it exists and --config is not set", func(t *testing.T) {
		t.Cleanup(viper.Reset)
		// This test is conditional on being able to create the file.
		// In some environments, this might fail due to permissions.
		const etcConfigPath = "/etc/ruf/config.yaml"
		err := os.MkdirAll(filepath.Dir(etcConfigPath), 0755)
		if err != nil {
			t.Skipf("Skipping test: could not create directory for %s: %v", etcConfigPath, err)
		}
		err = os.WriteFile(etcConfigPath, []byte("log:\n  level: warn"), 0644)
		if err != nil {
			t.Skipf("Skipping test: could not write to %s: %v", etcConfigPath, err)
		}
		defer os.RemoveAll(filepath.Dir(etcConfigPath))

		// Ensure --config flag is not set
		cfgFile = ""

		InitConfig()

		assert.Equal(t, etcConfigPath, viper.ConfigFileUsed())
		assert.Equal(t, "warn", viper.GetString("log.level"))
	})

	t.Run("uses XDG config path if --config and /etc/ruf/config.yaml are not present", func(t *testing.T) {
		t.Cleanup(viper.Reset)
		// Ensure /etc/ruf/config.yaml does not exist for this test
		os.RemoveAll("/etc/ruf")

		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)

		xdgRufDir := filepath.Join(tempDir, "ruf")
		err := os.MkdirAll(xdgRufDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create XDG dir: %v", err)
		}
		xdgConfigPath := filepath.Join(xdgRufDir, "config.yaml")
		err = os.WriteFile(xdgConfigPath, []byte("log:\n  level: error"), 0644)
		if err != nil {
			t.Fatalf("Failed to write XDG config: %v", err)
		}

		// Ensure --config flag is not set
		cfgFile = ""

		InitConfig()

		assert.Equal(t, xdgConfigPath, viper.ConfigFileUsed())
		assert.Equal(t, "error", viper.GetString("log.level"))
	})

	t.Run("proceeds without error if no config file is found", func(t *testing.T) {
		t.Cleanup(viper.Reset)
		// Ensure known locations are clean
		os.RemoveAll("/etc/ruf")
		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)

		// Ensure --config flag is not set
		cfgFile = ""

		// We just want to ensure this doesn't panic or error out.
		// The function itself logs a warning, which we can't easily capture here,
		// but we can check that viper doesn't have a config file set.
		assert.NotPanics(t, func() {
			InitConfig()
		})
		assert.Equal(t, "", viper.ConfigFileUsed())
	})
}
