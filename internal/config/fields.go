package config

import "reflect"

func Fields() []reflect.StructField { return configFields() }
