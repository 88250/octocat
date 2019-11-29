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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/88250/gulu"
	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
)

func pushRepos(c *gin.Context) {
	result := gulu.Ret.NewResult()
	result.Code = -1

	ak := c.PostForm("ak")
	user := user(ak)
	if nil == user {
		result.Msg = "get user failed"
		c.JSON(http.StatusOK, result)
		return
	}
	user["ak"] = ak

	file, err := c.FormFile("file")
	if nil != err {
		result.Msg = "get file failed"
		c.JSON(http.StatusOK, result)
		return
	}
	if 128 > file.Size {
		result.Msg = "file is too small"
		c.JSON(http.StatusOK, result)
		return
	}

	f, err := file.Open()
	if nil != err {
		result.Msg = "read file failed"
		c.JSON(http.StatusOK, result)
		return
	}
	fileData, err := ioutil.ReadAll(f)
	if nil != err {
		result.Msg = "read file data failed"
		c.JSON(http.StatusOK, result)
		return
	}

	repoName := c.PostForm("repoName")
	repoReadme := c.PostForm("repoReadme")
	repoDesc := c.PostForm("repoDesc")
	repoHomepage := c.PostForm("repoHomepage")
	favicon := c.PostForm("favicon")
	stat := c.PostForm("stat")

	owner := user["login"].(string)
	repoFullName := owner + "/" + repoName
	//logger.Infof("pushing repo [%s]", repoFullName)

	repo := createOrUpdateRepo(user, repoName, repoDesc, repoHomepage)
	if nil == repo {
		result.Msg = "create or update repo failed"
		c.JSON(http.StatusOK, result)
		return
	}

	repoReadme = strings.Replace(repoReadme, "${repoFullName}", repoFullName, -1)

	ok := updateFile(user, repoName, "README.md", []byte(repoReadme))
	if ok {
		ok = updateFile(user, repoName, "backup.zip", fileData)
	}
	if ok {
		result.Code = 0
	}

	c.JSON(http.StatusOK, result)

	var articleCnt int
	var recentArticleTime uint64
	statMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(stat), &statMap); nil == err {
		articleCnt = int(statMap["articleCount"].(float64))
		recentArticleTime = uint64(statMap["recentArticleTime"].(float64))
	}
	blogs.Store(repoFullName, &blog{repoDesc, repoHomepage, repoFullName, favicon, articleCnt, recentArticleTime})
}

func updateFile(user map[string]interface{}, repoName, filePath string, content []byte) (ok bool) {
	ak := user["ak"].(string)
	owner := user["login"].(string)
	fullRepoName := owner + "/" + repoName

	result := map[string]interface{}{}
	response, bytes, errors := gorequest.New().Get("https://api.github.com/repos/"+fullRepoName+"/git/trees/master?access_token="+ak).
		Set("User-Agent", UserAgent).Timeout(30 * time.Second).EndStruct(&result)
	if nil != errors {
		logger.Errorf("get git tree of file [%s] failed: %s", filePath, errors[0])
		return
	}
	if http.StatusOK != response.StatusCode && http.StatusConflict != response.StatusCode {
		logger.Errorf("get git tree of file [%s] status code [%d], body [%s]", filePath, response.StatusCode, string(bytes))
		return
	}

	body := map[string]interface{}{
		"message": ":memo: 更新博客",
		"content": base64.StdEncoding.EncodeToString(content),
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

	response, bytes, errors = gorequest.New().Put("https://api.github.com/repos/"+fullRepoName+"/contents/"+filePath+"?access_token="+ak).
		Set("User-Agent", UserAgent).Timeout(2 * time.Minute).
		SendMap(body).EndStruct(&result)
	if nil != errors {
		logger.Errorf("update repo [%s] file [%s] failed: %s", fullRepoName, filePath, errors[0])
		return
	}
	if http.StatusOK != response.StatusCode && http.StatusCreated != response.StatusCode {
		logger.Errorf("update repo [%s] file [%s] status code: %d, body: %s", fullRepoName, filePath, response.StatusCode, string(bytes))
		return
	}

	//logger.Infof("updated repo [%s] file [%s]", fullRepoName, filePath)

	return true
}

func createOrUpdateRepo(user map[string]interface{}, repoName, repoDesc, repoHomepage string) (repo map[string]interface{}) {
	body := map[string]interface{}{
		"name":        repoName,
		"description": repoDesc,
		"homepage":    repoHomepage,
		"has_wiki":    false,
	}

	ak := user["ak"].(string)

	response, bytes, errors := gorequest.New().Post("https://api.github.com/user/repos?access_token="+ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).EndStruct(&repo)
	if nil != errors {
		logger.Errorf("create repo failed: %v", errors[0])
		return nil
	}
	if http.StatusCreated != response.StatusCode && http.StatusUnprocessableEntity != response.StatusCode {
		logger.Errorf("create repo [%s] status code [%d], body [%s]", repo["full_name"], response.StatusCode, string(bytes))
		return nil
	}
	if http.StatusCreated == response.StatusCode {
		logger.Infof("created repo [%s]", repo["full_name"])
		return
	}

	owner := user["login"].(string)
	response, bytes, errors = gorequest.New().Patch("https://api.github.com/repos/"+owner+"/"+repoName+"?access_token="+ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).EndStruct(&repo)
	if nil != errors {
		logger.Errorf("update repo failed: %v", errors[0])
		return nil
	}
	if http.StatusOK != response.StatusCode {
		logger.Errorf("update repo [%s] status code [%d], body [%s]", repo["full_name"], response.StatusCode, string(bytes))
		return nil
	}

	logger.Infof("updated repo [%s]", repo["full_name"])
	return
}

func user(ak string) (ret map[string]interface{}) {
	response, bytes, errors := gorequest.New().Get("https://api.github.com/user?access_token="+ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).EndStruct(&ret)
	if nil != errors {
		logger.Errorf("get user failed: %v", errors[0])
		return nil
	}
	if http.StatusOK != response.StatusCode {
		logger.Errorf("get user status code [%d], body [%s]", response.StatusCode, string(bytes))
		return nil
	}
	return
}
