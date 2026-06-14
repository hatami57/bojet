module bojet

go 1.26.2

require (
	github.com/glebarez/sqlite v1.11.0
	github.com/hatami57/microjet/core v0.13.0
	github.com/hatami57/microjet/gormx v0.13.0
	github.com/hatami57/microjet/utils v0.13.0
	github.com/robfig/cron/v3 v3.0.1
	gopkg.in/telebot.v4 v4.0.0-beta.5
	gorm.io/gorm v1.31.1
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/uuid v1.5.0 // indirect
	github.com/hatami57/microjet/jsonx v0.12.0 // indirect
	github.com/hatami57/microjet/types v0.12.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.3.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	modernc.org/libc v1.37.6 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.7.2 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)

replace github.com/hatami57/microjet/core => ../microjet/core

replace github.com/hatami57/microjet/utils => ../microjet/utils

replace github.com/hatami57/microjet/gormx => ../microjet/gormx

replace github.com/hatami57/microjet/types => ../microjet/types

replace github.com/hatami57/microjet/jsonx => ../microjet/jsonx
