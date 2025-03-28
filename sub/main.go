package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	//file, err := os.OpenFile("sub.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer file.Close() // 关闭文件
	//log.SetOutput(file)

	getMqttClient()

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}

func getMqttClient() MQTT.Client {
	opts := MQTT.NewClientOptions().AddBroker(BROKER_HOST) // 创建 MQTT 客户端选项
	opts.SetUsername(USERNAME)                             // 设置用户名
	opts.SetPassword(PWD)                                  // 设置密码
	opts.SetClientID(CLIENTID)                             // 设置客户端ID
	opts.OnConnect = onConnnect                            // 设置连接处理器

	client := MQTT.NewClient(opts)                                       // 创建 MQTT 客户端实例
	if token := client.Connect(); token.Wait() && token.Error() != nil { // 连接到 MQTT 代理
		log.Println("can't connect to broker.")
		panic(token.Error())
	}
	return client
}

const (
	BROKER_HOST    = "tcp://localhost:1883"                                   // MQTT 代理服务器地址
	USERNAME       = "qrem-device-dev"                                        // MQTT 用户名
	PWD            = "qrem-device-dev"                                        // MQTT 密码
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
	HARDWARE_SUB_TOPIC        = "qrem/+/hardware"
	PRESSURE_PUB_TOPIC        = "qrem/%s/pressure_pad_test"
	PRODUCTION_TEST_PUB_TOPIC = "qrem/%s/production_test"
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

// 连接处理器函数
func onConnnect(client MQTT.Client) {
	log.Println("Connect to broker successed. ")
	// mac := "24C60018CSMX0028800000-00V1325"
	mac := "24C60011CSMX0028800000-00V1325"
	if t := client.Subscribe(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, controlMsgRecHandler); t.Wait() && t.Error() != nil {
		log.Println("Can't not subscribe " + HARDWARE_PUB_TOPIC + " topic.")
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

	if cmd == 0x70 {
		log.Println(hex.EncodeToString(decryptedData))
	}

	if strings.EqualFold("control", name) {
		// 版本号查询
		if cmd == 0xA0 {
			versionData, _ := hex.DecodeString("a004ff204d3030312d56312e332e30312d323032352d30312d31362031373a32383a3333030100010301000106010202010001")
			encryptedData, err := Encrypt(AES_CBC_5P, versionData, DefaultKey, DefaultIV)
			if err != nil {
				fmt.Println("Encrypt error:", err)
			}
			// 发布响应消息
			token := client.Publish(fmt.Sprintf("qrem/%s/server_ack", mac), 0, false, encryptedData)
			token.Wait()
			// 打印响应命令
			log.Println(fmt.Sprintf("public topic=server_ack,mac=%s,cmd=%X", mac, 0xA0))
		}
	}
	if strings.EqualFold("get_bed_status", name) {
		// 运行状态查询
		if cmd == 0xB4 {
		}
	}
}
