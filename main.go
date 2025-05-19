package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ipmiDimmTemp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ipmi_temp_dimm_sensor",
			Help: "IPMI DIMM sensor values",
		},
		[]string{"sensor_name"},
	)

	ipmiVRDimmTemp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ipmi_temp_vrdimm_sensor",
			Help: "IPMI VR DIMM sensor values",
		},
		[]string{"sensor_name"},
	)

	ipmiCPUTemp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ipmi_temp_cpu_sensor",
			Help: "IPMI CPU sensor values",
		},
		[]string{"sensor_name"},
	)

	ipmiEnvTemp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ipmi_temp_env_sensor",
			Help: "IPMI environment sensor values",
		},
		[]string{"sensor_name"},
	)

	HICTemp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ipmi_temp_hic_sensor",
			Help: "IPMI HIC sensor values",
		},
		[]string{"sensor_name"},
	)

	bnxtEnTemp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_card_temp_sensor",
			Help: "Network card temp sensor",
		},
		[]string{"sensor_name"},
	)
)

func init() {
	prometheus.MustRegister(ipmiDimmTemp, ipmiVRDimmTemp, ipmiCPUTemp, ipmiEnvTemp, HICTemp, bnxtEnTemp)
}

func collectMetrics() {
	collectIPMIMetrics()
	collectSensorMetrics()
}

func collectIPMIMetrics() {
	log.Println("Executing ipmitool command")
	out, err := exec.Command("sudo", "ipmitool", "sensor").Output()
	if err != nil {
		log.Printf("Failed to execute ipmitool: %v", err)
		return // Просто выходим из функции, не завершая приложение
	}

	output := string(out)
	log.Println("ipmitool output:")
	log.Println(output)

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		fields := strings.Split(line, "|")
		if len(fields) < 2 {
			continue
		}
		sensorName := strings.TrimSpace(fields[0])
		valueStr := strings.TrimSpace(fields[1])
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			log.Printf("Failed to parse sensor value for %s: %v", sensorName, err)
			continue
		}

		switch {
		case strings.Contains(sensorName, "DIMMG"):
			if strings.Contains(sensorName, "VR_DIMMG") {
				log.Printf("Setting VR DIMMG metric for %s: %f", sensorName, value)
				ipmiVRDimmTemp.With(prometheus.Labels{"sensor_name": sensorName}).Set(value)
			} else {
				log.Printf("Setting DIMMG metric for %s: %f", sensorName, value)
				ipmiDimmTemp.With(prometheus.Labels{"sensor_name": sensorName}).Set(value)
			}
		case strings.Contains(sensorName, "CPU") && strings.Contains(sensorName, "TEMP"):
			log.Printf("Setting CPU TEMP metric for %s: %f", sensorName, value)
			ipmiCPUTemp.With(prometheus.Labels{"sensor_name": sensorName}).Set(value)
		case strings.Contains(sensorName, "M2_AMB_TEMP"):
			log.Printf("Setting M2_AMB_TEMP metric for %s: %f", sensorName, value)
			ipmiEnvTemp.With(prometheus.Labels{"sensor_name": sensorName}).Set(value)
		case strings.Contains(sensorName, "HIC_TEMP"):
			log.Printf("Setting HIC_TEMP metric for %s: %f", sensorName, value)
			HICTemp.With(prometheus.Labels{"sensor_name": sensorName}).Set(value)
		default:
			log.Printf("Unknown sensor %s with value %f", sensorName, value)
		}
	}
}

func collectSensorMetrics() {
	log.Println("Executing sensors command")
	output, err := exec.Command("sensors").Output()
	if err != nil {
		log.Fatalf("Failed to execute sensors: %v", err)
	}

	var currentDevice string

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "bnxt_en-pci-0200" {
			currentDevice = line
		} else if currentDevice == "bnxt_en-pci-0200" && strings.Contains(line, "temp1") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				tempStr := fields[1]
				tempStr = strings.Trim(tempStr, "+°C")
				temp, err := strconv.ParseFloat(tempStr, 64)
				if err != nil {
					log.Printf("Failed to parse temperature for device %s: %v", currentDevice, err)
					continue
				}
				log.Printf("Setting temperature for device %s: %f", currentDevice, temp)
				bnxtEnTemp.With(prometheus.Labels{"sensor_name": currentDevice}).Set(temp)
				currentDevice = ""
			}
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling /metrics request")
	collectMetrics()
	promhttp.Handler().ServeHTTP(w, r)
}

func main() {
	// Открываем файл для логирования
	logFile, err := os.OpenFile("/log/ipmitool-exporter.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	log.Println("Starting ipmi_node_exporter")
	http.HandleFunc("/metrics", handler)
	log.Fatal(http.ListenAndServe(":9101", nil))
}
