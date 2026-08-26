package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// --- Структуры данных пакета ---
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
	OriginalDevId int                `json:"original_dev_id"`
	RecTs         int64              `json:"rec_ts"`
	Data          jsonSensorDataRpro `json:"data"`
	IsSent        int8               `json:"is_sent"`
	SentTs        int                `json:"sent_ts"`
}

type jsonSensorDataRpro struct {
	Humidity     float64 `json:"humidity"`
	RssiLora     int16   `json:"rssi_lora"`
	Temperature  float64 `json:"temperature"`
	Voltage      float32 `json:"voltage"`
	DiffPressure float32 `json:"diff_pressure"`
	//	AtmPressure   float32 `json:"pressure"`
	AmbientLight  float32 `json:"ambient_light"`
	AdapterTypeId int     `json:"adapter_type_id"`
	Pressure      float32 `json:"pressure"`
	DryContact1   int     `json:"dry_contact1"`
	DryContact2   int     `json:"dry_contact2"`
	Co2Lvl        float32 `json:"co2"`
}

type Config struct {
	ServerAddress string   `json:"server_address"`
	Port          int      `json:"port"`
	Devices       []Device `json:"devices"`
	IntervalSec   int      `json:"interval_sec"`    // Интервал между посылками ОДНОГО устройства
	TestDurationM int      `json:"test_duration_m"` // Общая длительность теста
}

type Device struct {
	DevId int `json:"dev_id"`
}

func randomFloat(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

// createPacket теперь принимает конкретный DevId
func createPacket(devId int) []dataStructuresRpro {
	slice := dataStructuresRpro{
		ControllerTypeId: 1,
		ControllerId:     333,
		PacketTs:         time.Now().Unix(),
		InternalIp:       "192.168.1.50",
		DevUptime:        time.Duration(rand.Intn(86400)) * time.Second,
		ProtocolVersion:  3,
		Data: sensorDataRpro{
			DevSn:         devId, // Уникальный серийный номер сессии
			OriginalDevId: devId,
			RecTs:         time.Now().Unix(),
			Data: jsonSensorDataRpro{
				Humidity:      randomFloat(40.0, 60.0),
				RssiLora:      int16(-rand.Intn(100)),
				Temperature:   randomFloat(20.0, 26.0),
				Voltage:       float32(randomFloat(3.3, 4.2)),
				DiffPressure:  float32(randomFloat(1.3, 4.2)),
				Pressure:      float32(randomFloat(990, 1030)),
				AmbientLight:  float32(randomFloat(0, 1000)),
				AdapterTypeId: 1,
				DryContact2:   0,
				DryContact1:   0,
			},
			IsSent: 0,
			SentTs: 0,
		},
	}
	sl := append([]dataStructuresRpro{}, slice)
	fmt.Println(slice)
	return sl
}

func worker(id int, cfg Config, device Device, client *http.Client, wg *sync.WaitGroup, globalCtx context.Context) {
	defer wg.Done()

	ticker := time.NewTicker(time.Duration(cfg.IntervalSec) * time.Second)
	defer ticker.Stop()

	url := fmt.Sprintf("http://%s:%d/anemon_hw_broker/input", cfg.ServerAddress, cfg.Port)
	sendCount := 0

	for {
		select {
		case <-globalCtx.Done():
			fmt.Printf("[Девайс %d] Останавливаюсь.\n", device.DevId)
			return
		case <-ticker.C:
			packet := createPacket(device.DevId)

			// ВАЖНО: Marshal-им только одну структуру, а не срез!
			jsonBody, err := json.Marshal(packet)
			if err != nil {
				continue
			}

			reqCtx, cancel := context.WithTimeout(globalCtx, 10*time.Second)

			// 2. Передаем именно reqCtx в запрос
			req, err := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewReader(jsonBody))
			if err != nil {
				cancel() // Важно очистить ресурсы, если создание не удалось
				continue
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("MQTT", "data")
			req.Header.Set("Controller-Type", "1")

			resp, err := client.Do(req)

			if err != nil {
				fmt.Printf("[%s][D:%d W:%d]  %v\n", time.Now().Format("15:04:05"), device.DevId, id, err)
			} else {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				sendCount++
				fmt.Printf("[%s][D:%d W:%d] Пакет #%d отправлен. Код: %s\n",
					time.Now().Format("15:04:05"), device.DevId, id, sendCount, resp.Status)
			}
			cancel()
		}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	file, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Println("Ошибка чтения config.json:", err)
		return
	}

	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		fmt.Println("Неверный формат JSON:", err)
		return
	}

	if cfg.IntervalSec <= 0 {
		cfg.IntervalSec = 600
	}
	if cfg.TestDurationM <= 0 {
		cfg.TestDurationM = 60
	}

	//client := &http.Client{
	//	Timeout: time.Second * 10,
	//	Transport: &http.Transport{
	//		// Ключевые параметры: заставляем закрывать соединение сразу после запроса
	//		DisableKeepAlives: true,        // <--- ГЛАВНОЕ ИЗМЕНЕНИЕ
	//		MaxIdleConns:      0,           // Обнуляем пул
	//		IdleConnTimeout:   time.Second, // Если вдруг открылось, быстро закрыть
	//	},
	//}

	client := &http.Client{
		// Убираем общий Timeout, он мешает диагностике
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second, // Держать трубу живой полминуты
			}).DialContext,
		},
	}

	// Контекст для завершения всех задач по времени
	globalCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TestDurationM)*time.Minute)
	defer cancel()

	var wg sync.WaitGroup

	fmt.Printf("Запуск симуляции: %d физических устройств.\nКаждое шлет данные раз в %d сек.\nОбщий тест: %d мин.\n",
		len(cfg.Devices), cfg.IntervalSec, cfg.TestDurationM)

	// Главный цикл: запускаем по одной горутине НА КАЖДОЕ устройство из конфига
	for i, dev := range cfg.Devices {
		wg.Add(1)
		// Передаем конкретное устройство в его персональную горутину
		go worker(i, cfg, dev, client, &wg, globalCtx)

		// Небольшая задержка при старте, чтобы воркеры не выстрелили первым пакетом строго одновременно
		time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
	}

	// Ждем либо окончания времени теста, либо сигнала прерывания (Ctrl+C)
	<-globalCtx.Done()
	fmt.Println("\nОсновное время вышло. Дожидаемся отправки последних пакетов...")

	// Даем секунду на завершение текущих Do-запросов
	time.Sleep(1 * time.Second)
	cancel() // Сигналим всем тикерам остановиться
	wg.Wait()

	fmt.Println("Тест успешно завершен.")
}
