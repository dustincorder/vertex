package protocol

import (
	"embed"
	"fmt"
)

// Configuration packets are versioned protocol data captured from an unmodified
// Java 26.3 server. Vertex owns the codec and connection state machine around it.
//
//go:embed data/v777/*.bin
var configurationFS embed.FS

type ConfigPacket struct {
	ID   int32
	Body []byte
}

var configurationFiles = []struct {
	id   int32
	file string
}{
	{0x0d, "01_update_enabled_features.bin"},
	{0x0f, "02_select_known_packs.bin"},
	{0x07, "03_registry_data.bin"}, {0x07, "04_registry_data.bin"},
	{0x07, "05_registry_data.bin"}, {0x07, "06_registry_data.bin"},
	{0x07, "07_registry_data.bin"}, {0x07, "08_registry_data.bin"},
	{0x07, "09_registry_data.bin"}, {0x07, "10_registry_data.bin"},
	{0x07, "11_registry_data.bin"}, {0x07, "12_registry_data.bin"},
	{0x07, "13_registry_data.bin"}, {0x07, "14_registry_data.bin"},
	{0x07, "15_registry_data.bin"}, {0x07, "16_registry_data.bin"},
	{0x07, "17_registry_data.bin"}, {0x07, "18_registry_data.bin"},
	{0x07, "19_registry_data.bin"}, {0x07, "20_registry_data.bin"},
	{0x07, "21_registry_data.bin"}, {0x07, "22_registry_data.bin"},
	{0x07, "23_registry_data.bin"}, {0x07, "24_registry_data.bin"},
	{0x07, "25_registry_data.bin"}, {0x07, "26_registry_data.bin"},
	{0x07, "27_registry_data.bin"}, {0x07, "28_registry_data.bin"},
	{0x07, "29_registry_data.bin"}, {0x07, "30_registry_data.bin"},
	{0x07, "31_registry_data.bin"}, {0x07, "32_registry_data.bin"},
	{0x07, "33_registry_data.bin"}, {0x07, "34_registry_data.bin"},
	{0x0e, "35_update_tags.bin"},
}

func ConfigurationPackets() ([]ConfigPacket, error) {
	packets := make([]ConfigPacket, 0, len(configurationFiles))
	for _, entry := range configurationFiles {
		body, err := configurationFS.ReadFile("data/v777/" + entry.file)
		if err != nil {
			return nil, fmt.Errorf("read protocol data %s: %w", entry.file, err)
		}
		packets = append(packets, ConfigPacket{ID: entry.id, Body: body})
	}
	return packets, nil
}
