// Octocat - B3log 的 GitHub 仓库操作服务
// Copyright (c) 2019-present, b3log.org
//
// Lute is licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/88250/gulu"
	"github.com/gin-gonic/gin"
)

const (
	UserAgent = "Octocat/1.0.0; +https://github.com/88250/octocat"
)

var logger *gulu.Logger

func init() {
	rand.Seed(time.Now().Unix())

	gulu.Log.SetLevel("info")
	logger = gulu.Log.NewLogger(os.Stdout)
	gin.SetMode(gin.ReleaseMode)
}

func mapRoutes() *gin.Engine {
	ret := gin.New()
	ret.Use(gin.Recovery())

	ret.POST("/github/repos/solo", pushRepos)
	ret.GET("/awesome-solo", refreshAwesomeSolo)
	ret.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "The piper will lead us to reason.\n\n欢迎访问黑客与画家的社区 https://hacpai.com")
	})

	return ret
}

func main() {
	orgAkArg := flag.String("ak", "", "")
	flag.Parse()
	if "" != *orgAkArg {
		orgAk = *orgAkArg
		fmt.Println("ak is [" + orgAk + "]")
	}

	go updateAwesomeSolo()

	router := mapRoutes()
	server := &http.Server{
		Addr:    "127.0.0.1:1123",
		Handler: router,
	}

	logger.Infof("octocat is running")
	server.ListenAndServe()
}
