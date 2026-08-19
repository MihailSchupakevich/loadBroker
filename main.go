package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

type dataStructuresRpro struct {
	ControllerTypeId int            `json:"controller_type_id"`
	ControllerId     int            `json:"controller_id"`
	PacketTs         int64          `json:"packet_ts"`
	InternalIp       string         `json:"internal_ip"`
	DevUptime        time.Duration  `json:"dev_uptime"`
	ProtocolVersion  uint8          `json:"protocol_version"`
	Data             sensorDataRpro `json:"data"`
}

type sensorDataRpro struct {
	DevSn         int                `json:"dev_sn"`
	OriginalDevId int                `json:"original_dev_id"` //Пользовательский номер радиомодуля\RS-485 адаптера
	RecTs         int64              `json:"rec_ts"`          //когда была сделана запись в базу.
	Data          jsonSensorDataRpro `json:"data"`
	IsSent        int8               `json:"is_sent"` //отправлена ли запись на сервер.-
	SentTs        int                `json:"sent_ts"` //timestamp когда была отправлена запись.

}

type jsonSensorDataRpro struct {
	DeltaV        int8    `json:"delta_v"`
	FromLast      int16   `json:"from_last"`
	Humidity      float64 `json:"humidity"`
	RssiLora      int16   `json:"rssi"`
	Temperature   float64 `json:"temperature"`
	Voltage       float32 `json:"voltage"`
	DiffPressure  float32 `json:"diff_pressure"`
	AtmPressure   float32 `json:"pressure"`
	DryContact1   int     `json:"dry_contact1"`
	DryContact2   int     `json:"dry_contact2"`
	DryContact    int     `json:"dry_contact"`
	AmbientLight  float32 `json:"ambient_light"`
	Co2Lvl        float32 `json:"co2"`
	AdapterTypeId int     `json:"adapter_type_id"`
}

type Config struct {
	ServerAddress     string   `json:"server_address"`
	Port              int      `json:"port"`
	Devices           []Device `json:"devices"`
	Concurrency       int      `json:"concurrency"`
	RequestsPerWorker int      `json:"requests_per_worker"`
}

type Device struct {
	Id    int `json:"id"`
	DevId int `json:"dev_id"`
}

func main() {

	file, errFile := os.ReadFile("config.json")
	if errFile != nil {
		return
	}

	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		fmt.Println("Неверный формат JSON:", err)
		return
	}

	client := &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}
	var wg sync.WaitGroup
	totalRequests := cfg.RequestsPerWorker * cfg.Concurrency
	fmt.Println(totalRequests)

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go worker(i, client, &wg, cfg)
	}
	wg.Wait()
	duration := time.Since(time.Now()).Seconds()
	rps := float64(totalRequests) / duration
	fmt.Printf("\n--- Тест завершен ---\nВремя: %.2f сек\nСредний RPS: %.2f\n", duration, rps)
}

func randomFloat(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func createPacket(devId int) dataStructuresRpro {
	return dataStructuresRpro{
		ControllerTypeId: 1,
		ControllerId:     devId,
		PacketTs:         time.Now().UnixMilli(),
		InternalIp:       "192.168.1.50",
		DevUptime:        time.Duration(rand.Intn(86400)) * time.Second,
		ProtocolVersion:  1,
		Data: sensorDataRpro{
			DevSn:         1001,
			OriginalDevId: devId,
			RecTs:         time.Now().UnixMilli(),
			Data: jsonSensorDataRpro{
				DeltaV:        int8(rand.Intn(10) - 5), // От -5 до 4
				FromLast:      int16(rand.Intn(500)),
				Humidity:      randomFloat(40.0, 60.0), // Диапазон влажности
				RssiLora:      int16(-rand.Intn(100)),  // Сигнал всегда отрицательный
				Temperature:   randomFloat(20.0, 26.0), // Диапазон температуры
				Voltage:       float32(randomFloat(3.3, 4.2)),
				DiffPressure:  float32(randomFloat(0, 50)),
				AtmPressure:   float32(randomFloat(990, 1030)),
				DryContact1:   rand.Intn(2),
				DryContact2:   rand.Intn(2),
				DryContact:    rand.Intn(2),
				AmbientLight:  float32(randomFloat(0, 1000)),
				Co2Lvl:        float32(randomFloat(400, 1200)),
				AdapterTypeId: 1,
			},
			IsSent: 0,
			SentTs: 0,
		},
	}
}
func worker(id int, client *http.Client, wg *sync.WaitGroup, cfg Config) {

	defer wg.Done()

	var sliceDevices []int

	for _, dev := range cfg.Devices {
		sliceDevices = append(sliceDevices, dev.DevId)
	}
	body := make([]dataStructuresRpro, 0, len(sliceDevices))

	for _, devId := range sliceDevices {
		body = append(body, createPacket(devId))
	}

	bodyRequest, errBodyRequest := json.Marshal(body)
	if errBodyRequest != nil {
		fmt.Println(errBodyRequest)
	}
	bodyRequestMarshal := bytes.NewReader(bodyRequest)

	req, errRequest := http.NewRequest("POST", fmt.Sprintf("http://%s:%d/anemon_hw_broker/input", cfg.ServerAddress, cfg.Port), bodyRequestMarshal)
	if errRequest != nil {
		fmt.Println(errRequest)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MQTT", "data")
	req.Header.Set("Controller-Type", "1")
	//key := cnf.RcvId * salt
	//req.Header.Set("X-API-key", fmt.Sprintf("%d", key))

	response, errDo := client.Do(req)
	if errDo != nil {
		fmt.Println(errDo)
		return
	}
	defer response.Body.Close()

	respText, _ := io.ReadAll(response.Body)
	fmt.Printf("Статус: %s\nОтвет сервера: %s\n", response.Status, string(respText))

}
