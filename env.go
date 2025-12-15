package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func GetEnv(key string) string {
	val, found := envVarMap[key]
	if found {
		return val
	}
	val = os.Getenv(key)
	return val
}

func MustGetEnv(key string) string {
	val := GetEnv(key)
	if strings.Trim(val, " ") == "" {
		panic(fmt.Errorf("environment variable for '%s' has not been set", key))
	}
	return val
}

func MustGetEnvInt(key string) int {
	v := MustGetEnv(key)
	i, err := strconv.Atoi(v)
	if err != nil {
		panic(err)
	}
	return i
}

func MustGetEnvBool(key string) bool {
	v := MustGetEnv(key)
	i, err := strconv.ParseBool(v)
	if err != nil {
		panic(err)
	}
	return i
}

func MustGetEnvMap(envVar string) map[string]string {
	var result map[string]string
	val := MustGetEnv(envVar)
	err := json.Unmarshal([]byte(val), &result)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal environment variable %s: %w", envVar, err))
	}

	return result
}

func MustGetEnvFloat(key string) float64 {
	v := MustGetEnv(key)
	i, err := strconv.ParseFloat(v, 64)
	if err != nil {
		panic(err)
	}
	return i
}
