package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/tidusant/c3m-common/log"
)

func init() {

}

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Indicates if debug messages should be printed in log files")
	flag.Parse()
	// logLevel := log.DebugLevel
	// if !debug {
	// 	logLevel = log.InfoLevel
	// 	gin.SetMode(gin.ReleaseMode)
	// }

	// log.SetOutputFile(fmt.Sprintf("build"), logLevel)
	// defer log.CloseOutputFile()
	// log.RedirectStdOut()

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	fmt.Println(exPath)

	done := make(chan bool)

	builderwait := viper.GetInt64("config.builderwait")
	numofbuild := viper.GetInt("config.nob")

	//upload file allow
	filetypeAllows := strings.Split(viper.GetString("config.zipfileextallow"), ",")
	filetypeAllowMap = make(map[string]string)
	for _, ext := range filetypeAllows {
		filetypeAllowMap[ext] = ext
	}

	//create 10 builder
	for i := 0; i < numofbuild; i++ {
		go func(i int) {
			for {

				time.Sleep(time.Second * time.Duration(builderwait))
				var builder Builder
				builder.Run()
				if builder.logtxt != "" {
					log.Debugf("finish run,log:%s", builder.logtxt)
				}
			}
		}(i)
	}

	// builderc.Run()
	// builderc.Run()
	// builderc.Run()
	// builderc.Run()
	<-done // Block forever
}
