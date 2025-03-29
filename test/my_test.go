package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func BenchmarkStructOperation2(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var demo struct {
			Name string
			Age  int
		}
		demo.Name = "Tom"
		demo.Age = randInt(1, 20)
		jsonStr, _ := json.Marshal(demo)
		fmt.Println(string(jsonStr))
	}
}
