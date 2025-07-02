package config

import "errors"

// Configuration loading errors
var (
	ErrConfigFileRead  = errors.New("failed to read config file")
	ErrConfigUnmarshal = errors.New("failed to unmarshal config")
)

// Configuration validation errors
var (
	ErrConfigNotPointer     = errors.New("config must be a non-nil pointer")
	ErrConfigNotStruct      = errors.New("config must be a pointer to struct")
	ErrConfigFieldNotSet    = errors.New("ConfigFile field is not settable")
	ErrConfigFieldNotString = errors.New("ConfigFile field must be a string")
)
