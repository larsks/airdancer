package api

import (
	"os"
	"strconv"
)

func getenvWithDefault(name string, defaultValue string) string {
	if val, ok := os.LookupEnv(name); ok {
		return val
	}
	return defaultValue
}

func getenvWithDefaultInt(name string, defaultValue int) int {
	if val, ok := os.LookupEnv(name); ok {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}
