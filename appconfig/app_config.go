package appconfig

import "github.com/SaiNageswarS/go-api-boot/config"

type AppConfig struct {
	config.BootConfig `ini:",extends"`

	EnableSearchSummarization bool `ini:"enable_search_summarization"`
}
