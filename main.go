package main

import (
	"fmt"
	"net/http"
	"os"
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

func main() {

	file, errFile := os.ReadFile("config.json")
	if errFile != nil {
		return
	}

	client := http.Client{}

	for i := range file {

	}

	request := http.Request{
		Method: "POST",
		Body:   file,
	}

	response, _ := client.Do(&request)
	defer response.Body.Close()

}
