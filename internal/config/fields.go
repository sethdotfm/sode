package config

import "reflect"

// Fields lists Config's fields with the embedded Router's in place of Router itself.
func Fields() []reflect.StructField { return configFields() }
