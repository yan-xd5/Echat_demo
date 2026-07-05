package main

import "echat/sdk/idgen"

// IDGen 别名，指向 SDK Snowflake。
type IDGen = idgen.Snowflake

// NewIDGen 创建 IDGen。
var NewIDGen = idgen.NewSnowflake
