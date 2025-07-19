package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Configurable represents a type that can be configured via flags and config files.
type Configurable interface {
	// AddFlags should add command-line flags to the provided FlagSet
	AddFlags(fs *pflag.FlagSet)
}

// ConfigLoader provides common configuration loading functionality.
type ConfigLoader struct {
	configFile   string
	defaults     map[string]any
	preserveFile bool
	strictMode   bool
}

// NewConfigLoader creates a new ConfigLoader instance.
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		defaults:     make(map[string]any),
		preserveFile: true,
		strictMode:   false,
	}
}

// SetConfigFile sets the configuration file path.
func (cl *ConfigLoader) SetConfigFile(configFile string) {
	cl.configFile = configFile
}

// SetDefault sets a default value for a configuration key.
func (cl *ConfigLoader) SetDefault(key string, value any) {
	cl.defaults[key] = value
}

// SetDefaults sets multiple default values at once.
func (cl *ConfigLoader) SetDefaults(defaults map[string]any) {
	for key, value := range defaults {
		cl.defaults[key] = value
	}
}

// SetStrictMode enables or disables strict mode for configuration validation.
// In strict mode, unknown configuration fields will cause an error.
func (cl *ConfigLoader) SetStrictMode(strict bool) {
	cl.strictMode = strict
}

// LoadConfig loads configuration with proper precedence: defaults < config file < explicit flags.
// The config parameter should be a pointer to the configuration struct to populate.
func (cl *ConfigLoader) LoadConfig(config any) error {
	return cl.LoadConfigWithFlagSet(config, pflag.CommandLine)
}

// LoadConfigWithFlagSet loads configuration with proper precedence using a custom flag set.
// The config parameter should be a pointer to the configuration struct to populate.
func (cl *ConfigLoader) LoadConfigWithFlagSet(config any, fs *pflag.FlagSet) error {
	// Store configFile if we need to preserve it (for structs that have ConfigFile field)
	var originalConfigFile string
	if cl.preserveFile && cl.configFile != "" {
		originalConfigFile = cl.configFile
	}

	v := viper.New()

	// Set default values
	for key, value := range cl.defaults {
		v.SetDefault(key, value)
	}

	// Read config file first (this overrides defaults)
	if cl.configFile != "" {
		v.SetConfigFile(cl.configFile)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("%w %s: %v", ErrConfigFileRead, cl.configFile, err)
		}

		// Apply environment variable substitution to config values
		cl.expandEnvironmentVariables(v)
	}

	// Only override with flags that were explicitly set by the user
	// This preserves the precedence: defaults < config file < explicit flags
	fs.Visit(func(flag *pflag.Flag) {
		// Keep flag names as-is since we now use hyphens in mapstructure tags
		// This handles cases like --dummy.switch-count -> dummy.switch-count
		viperKey := flag.Name

		// Get the actual value rather than string representation
		// This handles different flag types properly (int, uint, bool, etc.)
		if flagValue := flag.Value; flagValue != nil {
			// Try to get the underlying value for proper type handling
			switch flag.Value.Type() {
			case "uint", "uint8", "uint16", "uint32", "uint64":
				if val, err := strconv.ParseUint(flag.Value.String(), 10, 64); err == nil {
					v.Set(viperKey, val)
				} else {
					v.Set(viperKey, flag.Value.String())
				}
			case "int", "int8", "int16", "int32", "int64":
				if val, err := strconv.ParseInt(flag.Value.String(), 10, 64); err == nil {
					v.Set(viperKey, val)
				} else {
					v.Set(viperKey, flag.Value.String())
				}
			case "bool":
				if val, err := strconv.ParseBool(flag.Value.String()); err == nil {
					v.Set(viperKey, val)
				} else {
					v.Set(viperKey, flag.Value.String())
				}
			case "float32", "float64":
				if val, err := strconv.ParseFloat(flag.Value.String(), 64); err == nil {
					v.Set(viperKey, val)
				} else {
					v.Set(viperKey, flag.Value.String())
				}
			case "stringSlice":
				// Handle StringSliceVar flags properly by getting the actual slice value
				// instead of using String() which returns "[item1 item2]" format
				if sliceFlag, ok := flag.Value.(pflag.SliceValue); ok {
					v.Set(viperKey, sliceFlag.GetSlice())
				} else {
					// Fallback: try to parse the string representation
					// This handles the case where String() returns "[item1 item2]" format
					str := flag.Value.String()
					if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
						// Remove brackets and split by spaces
						str = strings.Trim(str, "[]")
						if str == "" {
							v.Set(viperKey, []string{})
						} else {
							items := strings.Fields(str)
							v.Set(viperKey, items)
						}
					} else {
						// Comma-separated format
						if str == "" {
							v.Set(viperKey, []string{})
						} else {
							items := strings.Split(str, ",")
							for i, item := range items {
								items[i] = strings.TrimSpace(item)
							}
							v.Set(viperKey, items)
						}
					}
				}
			default:
				// String or other type
				v.Set(viperKey, flag.Value.String())
			}
		}
	})

	// Use strict mode if enabled to detect unknown configuration fields
	if cl.strictMode {
		// Use strict mode if enabled to detect unknown configuration fields
		var unmarshalConfig mapstructure.DecoderConfig
		unmarshalConfig.Result = config
		unmarshalConfig.ErrorUnused = true
		unmarshalConfig.TagName = "mapstructure"
		unmarshalConfig.WeaklyTypedInput = true

		decoder, err := mapstructure.NewDecoder(&unmarshalConfig)
		if err != nil {
			return fmt.Errorf("%w: failed to create decoder: %v", ErrConfigUnmarshal, err)
		}

		if err := decoder.Decode(v.AllSettings()); err != nil {
			// Enhance the error message to include the config file name for better context
			if cl.configFile != "" {
				// Check if this is an "unused keys" error and enhance it with file context
				errStr := err.Error()
				if strings.Contains(errStr, "has invalid keys:") {
					// Replace the empty quotes with the config file name
					enhancedErr := strings.Replace(errStr, "* ''", fmt.Sprintf("* '%s'", cl.configFile), 1)
					return fmt.Errorf("%w: %s", ErrConfigUnmarshal, enhancedErr)
				}
			}
			return fmt.Errorf("%w: %v", ErrConfigUnmarshal, err)
		}
	} else {
		// Use regular unmarshaling which ignores unknown fields
		decodeHook := func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
			if f.Kind() != reflect.Struct {
				return data, nil
			}

			dataVal := reflect.ValueOf(data)
			if dataVal.IsZero() {
				return nil, nil
			}

			return data, nil
		}

		if err := v.Unmarshal(config, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			decodeHook,
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		))); err != nil {
			return fmt.Errorf("%w: %v", ErrConfigUnmarshal, err)
		}
	}

	// Restore configFile after unmarshal if it was set (prevents viper from clearing it)
	if cl.preserveFile && originalConfigFile != "" {
		// Use reflection to set ConfigFile field if it exists
		if err := cl.setConfigFileField(config, originalConfigFile); err != nil { //nolint:staticcheck
			// If we can't set it via reflection, that's okay - not all configs have this field
		}
	}

	return nil
}

// setConfigFileField attempts to set a ConfigFile field on the config struct using reflection.
func (cl *ConfigLoader) setConfigFileField(config any, configFile string) error {
	v := reflect.ValueOf(config)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("%w: got %T", ErrConfigNotPointer, config)
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("%w: got %s", ErrConfigNotStruct, v.Kind())
	}

	// Look for a field named "ConfigFile"
	field := v.FieldByName("ConfigFile")
	if !field.IsValid() {
		// Field doesn't exist, which is fine
		return nil
	}

	if !field.CanSet() {
		return fmt.Errorf("%w: ConfigFile", ErrConfigFieldNotSet)
	}

	if field.Kind() != reflect.String {
		return fmt.Errorf("%w: ConfigFile is %s", ErrConfigFieldNotString, field.Kind())
	}

	field.SetString(configFile)
	return nil
}

// StandardConfigPattern provides a convenient way to implement the standard config pattern.
func StandardConfigPattern(config Configurable, configFile string, defaults map[string]any) error {
	loader := NewConfigLoader()
	loader.SetConfigFile(configFile)
	if defaults != nil {
		loader.SetDefaults(defaults)
	}

	return loader.LoadConfig(config)
}

// LoadConfigWithFile is a convenience function that loads config from a specific file.
// This is useful for the monitor pattern where configFile is passed as a parameter.
func LoadConfigWithFile(config Configurable, configFile string, defaults map[string]any) error {
	return StandardConfigPattern(config, configFile, defaults)
}

// expandEnvironmentVariables recursively processes all string values in viper configuration
// and expands environment variable references using os.ExpandEnv
func (cl *ConfigLoader) expandEnvironmentVariables(v *viper.Viper) {
	// Get all settings as a map
	settings := v.AllSettings()

	// Process the settings recursively
	cl.expandMapValues(settings)

	// Set the expanded values back to viper
	for key, value := range settings {
		v.Set(key, value)
	}
}

// expandMapValues recursively processes map values and expands environment variables in strings
func (cl *ConfigLoader) expandMapValues(m map[string]any) {
	for key, value := range m {
		m[key] = cl.expandValue(value)
	}
}

// expandValue processes a single value and expands environment variables if it's a string
func (cl *ConfigLoader) expandValue(value any) any {
	switch v := value.(type) {
	case string:
		// Custom expansion that preserves original value if env var is not set
		return cl.expandString(v)
	case map[string]any:
		cl.expandMapValues(v)
		return v
	case []any:
		for i, item := range v {
			v[i] = cl.expandValue(item)
		}
		return v
	case map[any]any:
		// Convert to map[string]any and process
		converted := make(map[string]any)
		for k, val := range v {
			if strKey, ok := k.(string); ok {
				converted[strKey] = cl.expandValue(val)
			} else {
				converted[fmt.Sprintf("%v", k)] = cl.expandValue(val)
			}
		}
		return converted
	default:
		return value
	}
}

// expandString expands environment variables while preserving original text for unset variables
func (cl *ConfigLoader) expandString(s string) string {
	return os.Expand(s, func(key string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		// Return the original ${VAR} format if environment variable is not set
		return "${" + key + "}"
	})
}
