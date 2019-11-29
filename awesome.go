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
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/88250/gulu"
	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
	"github.com/parnurzeal/gorequest"
)

var blogs = &sync.Map{} // "88250/solo-blog" -> *blog

type blog struct {
	title             string // D 的个人博客 - 开源程序员，自由职业者
	homepage          string // https://88250.b3log.org
	repo              string // 88250/solo-blog
	favicon           string
	articleCnt        int
	recentArticleTime uint64
}

type blogSlice []*blog

func (s blogSlice) Len() int           { return len(s) }
func (s blogSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s blogSlice) Less(i, j int) bool { return s[i].recentArticleTime > s[j].recentArticleTime }

var orgAk = ""

var period = time.Hour * 6

func updateAwesomeSolo() {
	if 1 > len(orgAk) {
		return
	}

	for range time.Tick(period) {
		updateAwesomeSoloNow()
	}
}

func updateAwesomeSoloNow() {
	defer gulu.Panic.Recover(nil)
	ok, blogCount, articleCount := updateAwesomeSoloReadme()
	if ok {
		updateAwesomeSoloRepo(blogCount, articleCount)
	}
}

func updateAwesomeSoloRepo(blogCount, articleCount int) {
	body := map[string]interface{}{
		"name":        "awesome-solo",
		"description": "🎸 展示大家漂亮的 Solo 博客！目前已收录 " + strconv.Itoa(blogCount) + " 个站点，共 " + strconv.Itoa(articleCount) + " 篇文章 📈",
		"has_wiki":    false,
		"has_issues":  true,
	}

	response, str, errors := gorequest.New().Patch("https://api.github.com/repos/88250/awesome-solo?access_token="+orgAk).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).End()
	if nil != errors {
		logger.Errorf("update repo [88250/awesome-solo] failed: %v", errors[0])
		return
	}
	if http.StatusOK != response.StatusCode {
		logger.Errorf("update repo [88250/awesome-solo] status code [%d], body [%s]", response.StatusCode, str)
		return
	}

	logger.Infof("updated repo [88250/awesome-solo]")
	return
}

func sortAwesomeSolo() (ret blogSlice) {
	blogs.Range(func(key, value interface{}) bool {
		blog := value.(*blog)
		ret = append(ret, blog)
		return true
	})
	sort.Sort(ret)
	for _, blog := range ret {
		fmt.Println(blog.title, blog.homepage, blog.recentArticleTime, blog.articleCnt)
	}
	return
}

func updateAwesomeSoloReadme() (ok bool, blogCount, articleCount int) {
	solos := sortAwesomeSolo()
	result := map[string]interface{}{}
	filePath := "README.md"
	content := "| 图标 | 标题 | 链接 | 文章 | 仓库 |\n"
	content += "| :---: | --- | --- | ---: | :---: |\n"
	for _, solo := range solos {
		title := solo.title
		document, err := goquery.NewDocumentFromReader(strings.NewReader(title))
		if nil == err {
			title = document.Text()
		}
		title = sanitize(title)
		runes := []rune(title)
		if 26 <= len(runes) {
			title = string(runes[:26])
		}
		title = strings.TrimSpace(title)
		if strings.HasSuffix(title, "-") {
			title = title[:len(title)-1]
			title = strings.TrimSpace(title)
		}
		if 1 > len(title) {
			continue
		}
		homepage := sanitize(solo.homepage)
		favicon := sanitize(solo.favicon)
		if 0 < len(favicon) {
			favicon = "<img src=\"" + favicon + "\" width=\"24px\"/>"
			favicon = strings.ReplaceAll(favicon, "/interlace/0", "")
		}
		if strings.Contains(favicon, "solo-") {
			favicon = ""
		}

		ac := fmt.Sprintf("%d", solo.articleCnt)
		if "0" == ac {
			ac = ""
		}
		content += "| " + favicon + " | " + title + " | " + homepage + "| " + ac + " | [:octocat:](https://github.com/" + solo.repo + ") |\n"
		blogCount++
		articleCount += solo.articleCnt
	}

	if 1 > blogCount {
		return
	}

	content += "\n注：\n\n"
	content += "* 展示顺序按发布文章时间降序排列\n"
	content += "* 通过 [Octocat](https://github.com/88250/octocat) 自动定时刷新，请勿 PR\n"

	logger.Info("[awesome-solo]'s README.md content is [" + content + "]")

	response, bytes, errors := gorequest.New().Get("https://api.github.com/repos/88250/awesome-solo/git/trees/master?access_token="+orgAk).
		Set("User-Agent", UserAgent).Timeout(30 * time.Second).EndStruct(&result)
	if nil != errors {
		logger.Errorf("get git tree of file [%s] failed: %s", filePath, errors[0])

		return
	}
	if http.StatusOK != response.StatusCode && http.StatusConflict != response.StatusCode {
		logger.Errorf("get git tree of file [%s] status code [%d], body [%s]", filePath, response.StatusCode, string(bytes))

		return
	}

	now := time.Now()
	pattern := "2006-01-02 15:04:05"
	body := map[string]interface{}{
		"message": ":memo: 定时更新 " + now.Format(pattern) + "，下次刷新时间为 " + now.Add(period).Format(pattern),
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
	}
	if http.StatusOK == response.StatusCode {
		tree := result["tree"].([]interface{})
		for _, f := range tree {
			file := f.(map[string]interface{})
			if filePath == file["path"].(string) {
				body["sha"] = file["sha"]
				break
			}
		}
	}

	response, bytes, errors = gorequest.New().Put("https://api.github.com/repos/88250/awesome-solo/contents/"+filePath+"?access_token="+orgAk).
		Set("User-Agent", UserAgent).Timeout(2 * time.Minute).
		SendMap(body).EndStruct(&result)
	if nil != errors {
		logger.Errorf("update repo [88250/awesome-solo] file [%s] failed: %s", filePath, errors[0])
		return
	}
	if http.StatusOK != response.StatusCode && http.StatusCreated != response.StatusCode {
		logger.Errorf("update repo [88250/awesome-solo] file [%s] status code: %d, body: %s", filePath, response.StatusCode, string(bytes))
		return
	}

	logger.Infof("updated repo [88250/awesome-solo] file [%s]", filePath)
	ok = true
	return
}

func refreshAwesomeSolo(c *gin.Context) {
	updateAwesomeSoloNow()
	c.Status(200)
}

func sanitize(str string) (ret string) {
	ret = bluemonday.UGCPolicy().Sanitize(str)
	ret = strings.ReplaceAll(ret, "\n", " ")
	ret = strings.ReplaceAll(ret, "|", "\\|")
	ret = strings.TrimSpace(ret)
	return
}
