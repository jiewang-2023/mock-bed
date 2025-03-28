/*
 * 依赖库：go get github.com/eclipse/paho.mqtt.golang
 */
package main

import (
	"bytes"
	_ "bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

const (
	BROKER_HOST    = "tcp://172.16.4.207:1883"                                // MQTT 代理服务器地址
	USERNAME       = "mock"                                                   // MQTT 用户名
	PWD            = "mock"                                                   // MQTT 密码
	CMD_TOPIC      = "CommandTopic"                                           // 命令主题
	RESPONSE_TOPIC = "ResponseTopic"                                          // 响应主题
	DATA_TOPIC     = "DataTopic"                                              // 数据主题
	PAYLOAD        = "{\"name\":\"mqtt-device-01\",\"randnum\":\"520.1314\"}" // 模拟负载数据

	RESP_CLIENTID             = "Mock-Device-Response-ID" // 响应客户端ID
	CLIENTID                  = "Mock-Device-ID"          // 设备客户端ID
	RUN_STATUS_TOPIC          = "run_status"
	CONTROL_SUB_TOPIC         = "qrem/+/control"
	GET_BED_STATUS_SUB_TOPIC  = "qrem/+/get_bed_status"
	HARDWARE_PUB_TOPIC        = "qrem/%s/hardware"
	SERVER_ACK_PUB_TOPIC      = "qrem/%s/server_ack"
	PRESSURE_PUB_TOPIC        = "qrem/%s/pressure_pad_test"
	PRODUCTION_TEST_PUB_TOPIC = "qrem/%s/production_test"
	BODY_INFO_PUB_TOPIC       = "qrem/%s/body_info"
)

const (
	AES_CBC_5P = "AES/CBC/PKCS5Padding"
	AES_CFB_5P = "AES/CFB/PKCS5Padding"
)

var (
	DefaultKey = []byte{
		113, 114, 101, 109, 45, 97, 101, 115, 45, 107, 101, 121, 45, 112, 119, 100,
		55, 88, 116, 109, 45, 97, 101, 85, 68, 90, 122, 109, 45, 97, 101, 115,
	}

	DefaultIV = []byte{
		113, 114, 101, 109, 45, 97, 101, 115, 45, 105, 118, 105, 118, 45, 49, 48,
	}
)

// PKCS7Padding 填充
func PKCS7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// PKCS7UnPadding 去除填充
func PKCS7UnPadding(data []byte) []byte {
	length := len(data)
	unPadding := int(data[length-1])
	return data[:(length - unPadding)]
}

// Encrypt AES加密
func Encrypt(mode string, content, key, iv []byte) ([]byte, error) {
	if content == nil || key == nil || iv == nil {
		return nil, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 填充
	blockSize := block.BlockSize()
	content = PKCS7Padding(content, blockSize)

	var crypted []byte
	switch mode {
	case AES_CBC_5P:
		blockMode := cipher.NewCBCEncrypter(block, iv)
		crypted = make([]byte, len(content))
		blockMode.CryptBlocks(crypted, content)
	case AES_CFB_5P:
		stream := cipher.NewCFBEncrypter(block, iv)
		crypted = make([]byte, len(content))
		stream.XORKeyStream(crypted, content)
	}

	return crypted, nil
}

// Decrypt AES解密
func Decrypt(mode string, content, key, iv []byte) ([]byte, error) {
	if content == nil || key == nil || iv == nil {
		return nil, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	var origData []byte
	switch mode {
	case AES_CBC_5P:
		blockMode := cipher.NewCBCDecrypter(block, iv)
		origData = make([]byte, len(content))
		blockMode.CryptBlocks(origData, content)
	case AES_CFB_5P:
		stream := cipher.NewCFBDecrypter(block, iv)
		origData = make([]byte, len(content))
		stream.XORKeyStream(origData, content)
	}

	// 去除填充
	return PKCS7UnPadding(origData), nil
}

var (
	active = "false"              // 定义一个全局变量，表示设备是否处于活跃状态
	msgCh  = make(chan string, 1) // 创建一个缓冲通道，用于发送活跃状态
)

// 定义消息接收处理器函数，这里没有具体实现
// var msgRecHandler MQTT.MessageHandler = ...
func main() {
	//  代码格式化
	// go install mvdan.cc/gofumpt@latest
	// gofumpt -l -w .

	bedNum := flag.Int("bedNum", 100, "number of beds")
	// 解析命令行参数
	flag.Parse()
	fmt.Println("bedNum:", *bedNum)

	file, err := os.OpenFile("info.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close() // 关闭文件
	log.SetOutput(file)

	// client := getMqttClient()

	var wg sync.WaitGroup
	wg.Add(1)

	macs := make([]string, 0)

	mqttClientMap := make(map[string]MQTT.Client)

	for i := range *bedNum {
		m := fmt.Sprintf("25MM111111110038100000-%d", i)
		macs = append(macs, m)
		mqttClientMap[m] = getMqttClient(m)
	}

	// go sendDataActiveServer(msgCh, mqttClientMap) // 在单独的 goroutine 中发送活跃状态数据
	// 发送心跳
	go sendHeartBeat(macs, mqttClientMap)
	go sendHardWareMotherboardTemperature(macs, mqttClientMap)
	go sendHardWareSolenoidValveTemperature(macs, mqttClientMap)
	go sendHardWareAirPumpCurrent(macs, mqttClientMap)
	go sendHardWarePressurePad(macs, mqttClientMap)
	go sendHardWareSolenoidValveCurrent(macs, mqttClientMap)
	go sendErrorCode(macs, mqttClientMap)
	go sendMPR(macs, mqttClientMap)
	go sendGET_HARDWARE_ALL_STATUS(macs, mqttClientMap)
	go sendGET_ALGOR_ALL_STATUS(macs, mqttClientMap)
	go send8E(macs, mqttClientMap)
	go sendMovement(macs, mqttClientMap)
	go sendPosture(macs, mqttClientMap)
	go sendBodyshape(macs, mqttClientMap)
	go sendAdaptive_active(macs, mqttClientMap)
	go sendHr_HRV_BR(macs, mqttClientMap, 0x01)
	go sendHr_HRV_BR(macs, mqttClientMap, 0x02)
	//for {
	//	time.Sleep(3 * time.Second) // 每3秒向通道发送一次活跃状态
	//	msgCh <- "true"
	//}

	wg.Wait()
}

func sendHr_HRV_BR(macs []string, mqttClientMap map[string]MQTT.Client, opt byte) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public send8E,mac=%s,cmd=%X", mac, 0x9a))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0x9a)
			buffer.WriteByte(opt)
			dataMap := make(map[string]any)
			dataMap["HR"] = randInt(60, 110)
			bytedata, _ := json.Marshal(dataMap)
			buffer.Write(bytedata)
			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			buffer2 := bytes.NewBuffer(make([]byte, 0))
			buffer2.WriteByte(0x9b)
			buffer2.WriteByte(opt)
			dataMap2 := make(map[string]any)
			dataMap2["HRV"] = randInt(0, 10)
			bytedata2, _ := json.Marshal(dataMap2)
			buffer2.Write(bytedata2)
			bs2 := buffer2.Bytes()
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()

			buffer3 := bytes.NewBuffer(make([]byte, 0))
			buffer3.WriteByte(0x9c)
			buffer3.WriteByte(opt)
			dataMap3 := make(map[string]any)
			dataMap3["BR"] = randInt(10, 30)
			bytedata3, _ := json.Marshal(dataMap3)
			buffer3.Write(bytedata3)
			bs3 := buffer3.Bytes()
			encryptedData3, _ := Encrypt(AES_CBC_5P, bs3, DefaultKey, DefaultIV)
			token3 := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData3)
			go token3.Wait()

		}
		time.Sleep(1 * time.Second)
	}
}

func sendAdaptive_active(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public send8E,mac=%s,cmd=%X", mac, 0x97))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0x97)
			buffer.WriteByte(0x01)
			jsonStr := `{"head": {"val": 20, "airbag": [0]}, "shoulder": {"val": 5, "airbag": [1]}, "back": {"val": 5, "airbag": [2, 3]}, "upper_waist": {"val": 40, "airbag": [4, 5]}, "lower_waist": {"val": 40, "airbag": [6]}, "hip": {"val": 5, "airbag": [7, 8, 9]}, "leg": {"val": 40, "airbag": [10, 11]}}`
			buffer.WriteString(jsonStr)
			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			buffer2 := bytes.NewBuffer(make([]byte, 0))
			buffer2.WriteByte(0x97)
			buffer2.WriteByte(0x02)
			jsonStr2 := `{"head": {"val": 20, "airbag": [0]}, "shoulder": {"val": 5, "airbag": [1]}, "back": {"val": 5, "airbag": [2, 3]}, "upper_waist": {"val": 40, "airbag": [4, 5]}, "lower_waist": {"val": 40, "airbag": [6]}, "hip": {"val": 5, "airbag": [7, 8, 9]}, "leg": {"val": 40, "airbag": [10, 11]}}`
			buffer2.WriteString(jsonStr2)
			bs2 := buffer2.Bytes()
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}
		time.Sleep(7 * time.Second)
	}
}

func sendBodyshape(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public send8E,mac=%s,cmd=%X", mac, 0x8E))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0x95)
			buffer.WriteByte(0x01)
			jsonStr := `{"number": 59, "spine_x": [0, 2.0, 4.0, 5.99, 7.99, 9.99, 11.99, 13.98, 15.98, 17.98, 19.98, 21.98, 23.97, 25.97, 27.96, 29.96, 31.95, 33.94, 35.94, 37.93, 39.93, 41.93, 43.92, 45.92, 47.9, 49.9, 51.87, 53.86, 55.86, 57.85, 59.85, 61.85, 63.85, 65.85, 67.83, 69.8, 71.76, 73.7, 75.64, 77.58, 79.55, 81.51, 83.5, 85.49, 87.49, 89.49, 91.49, 93.49, 95.49, 97.49, 99.49, 101.49, 103.49, 105.48, 107.48, 109.48, 111.48, 113.48, 115.48], "spine_y": [0, 0.01, 0.0, -0.16, -0.33, -0.37, -0.41, -0.47, -0.42, -0.43, -0.32, -0.31, -0.12, -0.0, 0.18, 0.31, 0.5, 0.67, 0.79, 0.97, 1.0, 1.09, 1.01, 0.92, 0.65, 0.48, 0.15, -0.01, -0.17, -0.28, -0.34, -0.4, -0.32, -0.17, 0.09, 0.44, 0.84, 1.31, 1.79, 2.28, 2.65, 3.02, 3.22, 3.39, 3.5, 3.59, 3.68, 3.75, 3.73, 3.74, 3.72, 3.69, 3.64, 3.58, 3.5, 3.46, 3.38, 3.33, 3.26], "peak_chest": 0.0, "peak_waist": 0.0, "peak_hip": 0.0}`
			buffer.WriteString(jsonStr)
			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			buffer2 := bytes.NewBuffer(make([]byte, 0))
			buffer2.WriteByte(0x95)
			buffer2.WriteByte(0x02)
			jsonStr2 := `{"number": 59, "spine_x": [0, 2.0, 4.0, 5.99, 7.99, 9.99, 11.99, 13.98, 15.98, 17.98, 19.98, 21.98, 23.97, 25.97, 27.96, 29.96, 31.95, 33.94, 35.94, 37.93, 39.93, 41.93, 43.92, 45.92, 47.9, 49.9, 51.87, 53.86, 55.86, 57.85, 59.85, 61.85, 63.85, 65.85, 67.83, 69.8, 71.76, 73.7, 75.64, 77.58, 79.55, 81.51, 83.5, 85.49, 87.49, 89.49, 91.49, 93.49, 95.49, 97.49, 99.49, 101.49, 103.49, 105.48, 107.48, 109.48, 111.48, 113.48, 115.48], "spine_y": [0, 0.01, 0.0, -0.16, -0.33, -0.37, -0.41, -0.47, -0.42, -0.43, -0.32, -0.31, -0.12, -0.0, 0.18, 0.31, 0.5, 0.67, 0.79, 0.97, 1.0, 1.09, 1.01, 0.92, 0.65, 0.48, 0.15, -0.01, -0.17, -0.28, -0.34, -0.4, -0.32, -0.17, 0.09, 0.44, 0.84, 1.31, 1.79, 2.28, 2.65, 3.02, 3.22, 3.39, 3.5, 3.59, 3.68, 3.75, 3.73, 3.74, 3.72, 3.69, 3.64, 3.58, 3.5, 3.46, 3.38, 3.33, 3.26], "peak_chest": 0.0, "peak_waist": 0.0, "peak_hip": 0.0}`
			buffer2.WriteString(jsonStr2)
			bs2 := buffer2.Bytes()
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}
		time.Sleep(1 * time.Second)
	}
}

func sendPosture(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public sendPosture,mac=%s,cmd=%X", mac, 0x91))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0x91)
			buffer.WriteByte(0x01)
			dataMap := make(map[string]any)
			dataMap["posture"] = randInt(0, 7)
			bytedata, _ := json.Marshal(dataMap)
			buffer.Write(bytedata)
			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			buffer2 := bytes.NewBuffer(make([]byte, 0))
			buffer2.WriteByte(0x91)
			buffer2.WriteByte(0x02)
			dataMap2 := make(map[string]any)
			dataMap2["posture"] = randInt(0, 7)
			bytedata2, _ := json.Marshal(dataMap2)
			buffer2.Write(bytedata2)
			bs2 := buffer2.Bytes()
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}
		time.Sleep(30 * time.Second)
	}
}

func sendMovement(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public sendMovement,mac=%s,cmd=%X", mac, 0x91))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0x91)
			buffer.WriteByte(0x01)
			dataMap := make(map[string]any)
			dataMap["movement"] = randInt(0, 2)
			bytedata, _ := json.Marshal(dataMap)
			buffer.Write(bytedata)
			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			buffer2 := bytes.NewBuffer(make([]byte, 0))
			buffer2.WriteByte(0x91)
			buffer2.WriteByte(0x02)
			dataMap2 := make(map[string]any)
			dataMap2["movement"] = randInt(0, 2)
			bytedata2, _ := json.Marshal(dataMap2)
			buffer2.Write(bytedata2)
			bs2 := buffer2.Bytes()
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}
		time.Sleep(5 * time.Second)
	}
}

func send8E(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public send8E,mac=%s,cmd=%X", mac, 0x8E))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0x8E)
			buffer.WriteByte(0x01)
			jsonStr := `{
	"supine": {
		"head": {
			"hit": [
				[3, 1],
				[4, -1]
			],
			"val": [20, -1]
		},
		"shoulder": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"back": {
			"hit": [],
			"val": [30, 0]
		},
		"upper_waist": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"lower_waist": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"hip": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"leg": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		}
	},
	"lateral": {
		"head": {
			"hit": [
				[3, 1],
				[4, -1]
			],
			"val": [20, -1]
		},
		"shoulder": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"back": {
			"hit": [],
			"val": [30, 0]
		},
		"upper_waist": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"lower_waist": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"hip": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		},
		"leg": {
			"hit": [
				[1, 1],
				[2, -1],
				[5, 1]
			],
			"val": [40, 1]
		}
	}
}`
			buffer.WriteString(jsonStr)
			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(BODY_INFO_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()
		}
		time.Sleep(30 * time.Second)
	}
}

func sendGET_ALGOR_ALL_STATUS(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public GET_ALGOR_ALL_STATUS,mac=%s,cmd=%X", mac, 0xb1))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0xb1)
			buffer.WriteByte(0x00)

			dataMap := make(map[string]any)
			dataMap["pillowFlag"] = 1
			dataMap["adaptiveMode"] = 1
			dataMap["shieldAdaptive"] = 1
			dataMap["floatingMode"] = 1
			dataMap["welcomeMode"] = 1
			dataMap["runStatus"] = 1
			dataMap["posture"] = 1
			dataMap["bedExitStatus"] = 1
			dataMap["bedModel"] = "EK-E"
			dataMap["firmwareVersion"] = "M001-V1.3.01-2025-01-16 17:28:33"
			dataMap["storage"] = "1024 MB"
			bytedata, _ := json.Marshal(dataMap)

			buffer.Write(bytedata)

			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(SERVER_ACK_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()
		}
		time.Sleep(15 * time.Second)
	}
}

func sendGET_HARDWARE_ALL_STATUS(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public GET_HARDWARE_ALL_STATUS,mac=%s,cmd=%X", mac, 0xb3))
			buffer := bytes.NewBuffer(make([]byte, 0))
			buffer.WriteByte(0xb3)
			buffer.WriteByte(0x00)
			buffer.WriteByte(0x01)
			buffer.WriteByte(0x05)
			buffer.WriteString("qrem_guestqrem_guestqrem_guest0")
			buffer.WriteByte(0x00)
			buffer.WriteByte(0x01)

			bs := buffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(SERVER_ACK_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()
		}
		time.Sleep(15 * time.Second)
	}
}

func sendMPR(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public sendMPR,mac=%s,cmd=%X", mac, 0x70))

			bs, _ := hex.DecodeString("700a0100199c230019a725001993a30024612900245273002460d8002451f400245b150024558d002462e40022bc9600245f1a00244a9a00245f6e001c69aa")
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			bs2, _ := hex.DecodeString("7009010019c3cc001e1a05001da12700263da7002619b000263c5e001d6bda001da1b80024752f00244b64002444330024b9a3001a18ae0019cb6d0019d4b7")
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func randInt(min, max int) int {
	return min + rand.Intn(max-min)
}

func sendErrorCode(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("send errorCode %s", mac))

			now := time.Now()
			yearStr := strconv.Itoa(now.Year())
			yearLastTwo, _ := strconv.Atoi(yearStr[len(yearStr)-2:])
			bs := []byte{0xec, 4, byte(randInt(0x01, 0x04)), 1, byte(randInt(0x01, 0x0f)), byte(yearLastTwo), byte(now.Month()), byte(now.Day()), byte(now.Hour()), byte(now.Minute()), byte(now.Second())}
			encryptedData, err := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			if err != nil {
				fmt.Println("Encrypt error:", err)
			}

			token := client.Publish(fmt.Sprintf(PRODUCTION_TEST_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()
		}
		time.Sleep(10 * time.Second)
	}
}

func sendHardWarePressurePad(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public topic=hardware,mac=%s,cmd=%X", mac, 0x71))

			newBuffer := bytes.NewBuffer(make([]byte, 0))
			newBuffer.WriteByte(0x71)
			newBuffer.WriteByte(0x01)
			for range 1024 {
				newBuffer.WriteByte(byte(randInt(0, 126)))
			}
			dataArr := newBuffer.Bytes()
			encryptedData, _ := Encrypt(AES_CBC_5P, dataArr, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(PRESSURE_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			newBuffer2 := bytes.NewBuffer(make([]byte, 0))
			newBuffer2.WriteByte(0x71)
			newBuffer2.WriteByte(0x02)
			for range 1024 {
				newBuffer2.WriteByte(byte(randInt(0, 80)))
			}
			dataArr2 := newBuffer2.Bytes()
			encryptedData2, _ := Encrypt(AES_CBC_5P, dataArr2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(PRESSURE_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}

		time.Sleep(72 * time.Millisecond)
	}
}

func sendHardWareAirPumpCurrent(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public topic=hardware,mac=%s,cmd=%X", mac, 0x73))

			bs := []byte{0x73, 4, byte(randInt(0, 1000)), byte(randInt(0, 1000)), 0, 0, byte(randInt(0, 1000)), byte(randInt(0, 1000))}
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()
		}
		time.Sleep(1 * time.Second)
	}
}

func sendHardWareSolenoidValveTemperature(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public topic=hardware,mac=%s,cmd=%X", mac, 0x75))

			bs := []byte{0x75, 1, byte(randInt(10, 60)), byte(randInt(10, 60)), byte(randInt(10, 60))}
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			bs2 := []byte{0x75, 2, byte(randInt(10, 80)), byte(randInt(10, 80)), byte(randInt(10, 80))}
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}
		time.Sleep(1 * time.Second)
	}
}

func sendHardWareSolenoidValveCurrent(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public topic=hardware,mac=%s,cmd=%X", mac, 0x75))

			bs, _ := hex.DecodeString("7401000000000000")
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			bs2, _ := hex.DecodeString("7402000000000000")
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			token2 := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData2)
			go token2.Wait()
		}
		time.Sleep(1 * time.Second)
	}
}

func sendHardWareMotherboardTemperature(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("public topic=hardware,mac=%s,cmd=%X", mac, 0x75))

			bs, _ := hex.DecodeString("7601018a01790121026b03ff")
			encryptedData, _ := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			token := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData)
			go token.Wait()

			bs2, _ := hex.DecodeString("76020241022b017c01f30267")
			encryptedData2, _ := Encrypt(AES_CBC_5P, bs2, DefaultKey, DefaultIV)
			t2 := client.Publish(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, false, encryptedData2)
			go func() {
				_ = t2.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
				if t2.Error() != nil {
					log.Println(t2.Error()) // Use your preferred logging technique (or just fmt.Printf)
				}
			}()
		}
		time.Sleep(3 * time.Second)
	}
}

func getMqttClient(clientId string) MQTT.Client {
	opts := MQTT.NewClientOptions().AddBroker(BROKER_HOST) // 创建 MQTT 客户端选项
	opts.SetUsername(USERNAME)                             // 设置用户名
	opts.SetPassword(PWD)                                  // 设置密码
	opts.SetClientID(clientId)                             // 设置客户端ID
	opts.OnConnect = onConnnect                            // 设置连接处理器

	client := MQTT.NewClient(opts)                                       // 创建 MQTT 客户端实例
	if token := client.Connect(); token.Wait() && token.Error() != nil { // 连接到 MQTT 代理
		log.Println("can't connect to broker.")
		panic(token.Error())
	}
	return client
}

func test() bool {
	// 加密示例
	originalData := "Hello, World!"
	encryptedData, err := Encrypt(AES_CBC_5P, []byte(originalData), DefaultKey, DefaultIV)
	if err != nil {
		fmt.Println("Encrypt error:", err)
		return true
	}
	fmt.Printf("Encrypted Data: %s\n", hex.EncodeToString(encryptedData))

	// 解密示例
	decryptedData, err := Decrypt(AES_CBC_5P, encryptedData, DefaultKey, DefaultIV)
	if err != nil {
		fmt.Println("Decrypt error:", err)
		return true
	}

	fmt.Printf("Decrypted Data: %s\n", string(decryptedData))
	return false
}

// 连接处理器函数
func onConnnect(client MQTT.Client) {
	log.Println("Connect to broker successed. ")
	if t := client.Subscribe(CONTROL_SUB_TOPIC, 0, controlMsgRecHandler); t.Wait() && t.Error() != nil {
		log.Println("Can't not subscribe " + CONTROL_SUB_TOPIC + " topic.")
		panic(t.Error())
	}
	if t := client.Subscribe(GET_BED_STATUS_SUB_TOPIC, 0, controlMsgRecHandler); t.Wait() && t.Error() != nil {
		log.Println("Can't not subscribe " + GET_BED_STATUS_SUB_TOPIC + " topic.")
		panic(t.Error())
	}
	log.Println("Start subscribe  topic.")
}

// 消息接收处理器函数
func controlMsgRecHandler(client MQTT.Client, msg MQTT.Message) {
	payload := msg.Payload()
	// log.Printf("Recv msg : %s\n", payload) // 打印接收到的消息
	topic := msg.Topic()
	topicItem := strings.Split(topic, "/")
	mac := topicItem[1]
	name := topicItem[2]

	decryptedData, err := Decrypt(AES_CBC_5P, payload, DefaultKey, DefaultIV)
	if err != nil {
		fmt.Println("Decrypt error:", err)
		return
	}
	buffer := bytes.NewBuffer(decryptedData)
	cmd, _ := buffer.ReadByte()
	opt, _ := buffer.ReadByte()
	log.Println(fmt.Sprintf("recv topic=%s,mac=%s,cmd=%X,opt=%X", name, mac, cmd, opt))

	if strings.EqualFold("control", name) {
		// 版本号查询
		if cmd == 0xA0 {
			versionData, _ := hex.DecodeString("a004ff204d3030312d56312e332e30312d323032352d30312d31362031373a32383a3333030100010301000106010202010001")
			encryptedData, err := Encrypt(AES_CBC_5P, versionData, DefaultKey, DefaultIV)
			if err != nil {
				fmt.Println("Encrypt error:", err)
			}
			// 发布响应消息
			t := client.Publish(fmt.Sprintf(SERVER_ACK_PUB_TOPIC, mac), 0, false, encryptedData)
			go func() {
				_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
				if t.Error() != nil {
					log.Println(t.Error()) // Use your preferred logging technique (or just fmt.Printf)
				}
				log.Println(fmt.Sprintf("public topic=server_ack,mac=%s,cmd=%X", mac, 0xA0))
			}()
		}
	}
	if strings.EqualFold("get_bed_status", name) {
		// 运行状态查询
		if cmd == 0xB4 {

			dataMap := make(map[string]int)
			dataMap["ddr"] = (randInt(50, 99))
			dataMap["cpu"] = (randInt(50, 99))
			dataMap["flash"] = (randInt(50, 99))
			bytedata, _ := json.Marshal(dataMap)

			byteArr := make([]byte, 0)
			newBuffer := bytes.NewBuffer(byteArr)
			newBuffer.WriteByte(0xB4)
			newBuffer.WriteByte(0x04)
			newBuffer.Write(bytedata)

			dataArr := newBuffer.Bytes()
			encryptedData, err := Encrypt(AES_CBC_5P, dataArr, DefaultKey, DefaultIV)
			if err != nil {
				fmt.Println("Encrypt error:", err)
			}
			t := client.Publish(fmt.Sprintf(SERVER_ACK_PUB_TOPIC, mac), 0, false, encryptedData) // 发布响应消息
			go func() {
				_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
				if t.Error() != nil {
					log.Println(t.Error()) // Use your preferred logging technique (or just fmt.Printf)
				}
				log.Println(fmt.Sprintf("public topic=server_ack,mac=%s,cmd=%X", mac, 0xB4)) // 打印响应命令
			}()

		}
	}
}

// 消息接收处理器函数
func msgRecHandler(client MQTT.Client, msg MQTT.Message) {
	log.Printf("Recv msg : %s\n", msg.Payload()) // 打印接收到的消息
	cmdMap := make(map[string]string)
	json.Unmarshal(msg.Payload(), &cmdMap) // 解析消息负载
	// topic := msg.Topic()

	cmd := cmdMap["cmd"]       // 获取命令
	method := cmdMap["method"] // 获取方法

	switch cmd {
	case "ping":
		cmdMap["ping"] = "pong" // 处理 ping 命令
	case "randnum":
		cmdMap["randnum"] = "520.1314" // 处理 randnum 命令
	case "message":
		if method == "get" {
			cmdMap["message"] = "Are you ok?" // 处理 message get 命令
		} else {
			cmdMap["result"] = "set successed." // 处理 message set 命令
		}
	case "collect":
		if method == "get" {
			cmdMap["collect"] = active // 处理 collect get 命令
		} else {
			cmdMap["result"] = "set successed."
			active = cmdMap["param"] // 更新活跃状态
		}
	}
	respMsg, err := json.Marshal(cmdMap) // 将命令映射序列化为 JSON
	if err != nil {
		log.Println(err)
	}
	token := client.Publish(RESPONSE_TOPIC, 0, false, respMsg) // 发布响应消息
	token.Wait()
	log.Println("Response cmd : " + string(respMsg)) // 打印响应命令
}

// func sendDataActiveServer(ch <-chan string,  client MQTT.Client) {
// 	for {
// 		select {
// 		case msg, ok := <-ch: // 从通道接收活跃状态
// 			if ok {
// 				active = msg
// 			}
// 		default:
// 			time.Sleep(100 * time.Millisecond) // 如果通道为空，则等待100毫秒
// 		}

// 		if active == "true" {
// 			log.Println("send data actively from mock device.")
// 			log.Println("         " + PAYLOAD)

// 			token := client.Publish(DATA_TOPIC, 0, false, []byte(PAYLOAD)) // 发布模拟负载数据
// 			token.Wait()
// 			time.Sleep(1 * time.Second) // 等待1秒
// 		}
// 	}
// }

// 发布心跳数据包
func sendHeartBeat(macs []string, mqttClientMap map[string]MQTT.Client) {
	for {
		for _, mac := range macs {
			client := mqttClientMap[mac]
			log.Println(fmt.Sprintf("send heartbeat %s", mac))

			bs := []byte{0x55, 4}
			encryptedData, err := Encrypt(AES_CBC_5P, bs, DefaultKey, DefaultIV)
			if err != nil {
				fmt.Println("Encrypt error:", err)
			}

			t := client.Publish("qrem/"+mac+"/"+RUN_STATUS_TOPIC, 0, false, encryptedData)
			go func() {
				_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
				if t.Error() != nil {
					log.Println(t.Error()) // Use your preferred logging technique (or just fmt.Printf)
				}
			}()
		}
		time.Sleep(10 * time.Second)
	}
}
