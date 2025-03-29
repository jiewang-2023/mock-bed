package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"mock-bed/pkg/encryption"
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
	BROKER_HOST        = "tcp://localhost:1883" // MQTT 代理服务器地址
	USERNAME           = "qrem-device-dev"      // MQTT 用户名
	PWD                = "qrem-device-dev"      // MQTT 密码
	CLIENTID           = "Mock-Device-ID"       // 设备客户端ID
	HARDWARE_PUB_TOPIC = "qrem/%s/hardware"
)

// 定义消息接收处理器函数，这里没有具体实现
// var msgRecHandler MQTT.MessageHandler = ...

// 连接处理器函数
func onConnnect(client MQTT.Client) {
	log.Println("Connect to broker successed. ")
	// mac := "24C60018CSMX0028800000-00V1325"
	mac := "24C60011CSMX0028800000-00V1325"
	if t := client.Subscribe(fmt.Sprintf(HARDWARE_PUB_TOPIC, mac), 0, controlMsgRecHandler); t.Wait() && t.Error() != nil {
		log.Println("Can't not subscribe " + fmt.Sprintf(HARDWARE_PUB_TOPIC, mac) + " topic.")
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

	decryptedData, err := encryption.Decrypt(payload)
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
			encryptedData, err := encryption.Encrypt(versionData)
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
