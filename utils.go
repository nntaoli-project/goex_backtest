package main

import "encoding/json"

func DeepCopyStruct(source, target interface{}) {
	data, _ := json.Marshal(source)
	json.Unmarshal(data, target)
}
