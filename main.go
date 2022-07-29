package main

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DoctorRegInfo struct {
	DoctorName  string `json:"doctorName"`
	DeptName    string `json:"deptName"`
	DoctorTitle string `json:"doctorTitle"`
	RegList     []struct {
		RegDate       string `json:"regDate"`
		ScheduleType  string `json:"scheduleType"`
		RegTotalCount string `json:"regTotalCount"`
		RegLeaveCount string `json:"regLeaveCount"`
		BabyRegFee    string `json:"babyRegFee"`
		BabyTreatFee  string `json:"babyTreatFee"`
		PassKey       string `json:"passKey"`
	} `json:"regList"`
}

type DoctorRegTimeInfo struct {
	BeginTime     string `json:"beginTime"`
	EndTime       string `json:"endTime"`
	RegTotalCount string `json:"regTotalCount"`
	RegLeaveCount string `json:"regLeaveCount"`
	ScheduleType  string `json:"scheduleType"`
	RadioEnable   bool   `json:"radioEnable"`
}

type Config struct {
	JSessionID       string `yaml:"Jid"`
	RegistrationInfo struct {
		HospitalID   string `yaml:"hospital_id"`
		DepartmentID string `yaml:"department_id"`
		DoctorID     string `yaml:"doctor_id"`
	} `yaml:"registration_info"`

	Display struct {
		DoctorRegistrationInfo     bool `yaml:"doctor_registration_info"`
		DoctorRegistrationTimeInfo bool `yaml:"doctor_registration_time_info"`
		DoctorValidRegistration    bool `yaml:"doctor_valid_registration"`
	} `yaml:"display"`

	Cron struct {
		Schedule     bool   `yaml:"schedule"`
		Pattern      string `yaml:"pattern"`
		BotWebHook   string `yaml:"bot_web_hook"`
		Date         string `yaml:"date"`
		FilterByTime bool   `yaml:"filter_by_time"`
		BeginTime    string `yaml:"begin_time"`
		EndTime      string `yaml:"end_time"`
	} `yaml:"cron"`
}

type WeworkBotContent struct {
	Msgtype string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

var sugar *zap.SugaredLogger
var client *http.Client
var config *Config
var cronQuit chan bool

const userAgent = "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63070517)"

func main() {

	initZap()
	initConfig()
	initClient()

	//test()

	//getCookie()
	if config.Cron.Schedule == false {
		findDoctorRegInfo()
	} else {

		initCron()
		//scheduleJobSendRemain()
	}

}

func test() {

	t := true

	if t == true {
		sugar.Warn("stop")
		cronQuit <- true
		sugar.Warn("after stop")

	} else {
		sugar.Warn("continue")
	}

}

func initZap() {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")

	config := zap.NewProductionConfig()
	config.Development = false
	config.Encoding = "console"
	config.DisableStacktrace = true
	config.EncoderConfig = encoderConfig

	logger, _ := config.Build()
	defer logger.Sync()

	sugar = logger.Sugar()
}

func initConfig() {
	f, err := ioutil.ReadFile("./config.yaml")
	if err != nil {
		sugar.Errorw("read file failure",
			"err", err.Error())
	}

	err = yaml.Unmarshal(f, &config)
	if err != nil {
		sugar.Errorw("unmarshal yaml failure",
			"err", err.Error())
	}
}

func initClient() {
	newJar, err := cookiejar.New(nil)
	if err != nil {
		sugar.Error()
	}

	client = new(http.Client)

	cookie := &http.Cookie{
		Name:   "JSESSIONID",
		Value:  config.JSessionID,
		Domain: "ihis.gzfezx.com",
	}

	cookies := make([]*http.Cookie, 0)
	cookies = append(cookies, cookie)

	cookieURL, _ := url.Parse("https://ihis.gzfezx.com/fyfwh-web/public/getDrRegTimeInfo001")
	newJar.SetCookies(cookieURL, cookies)

	client.Jar = newJar

}

func initCron() {
	sugar.Infof("starting go cron...")

	cronQuit = make(chan bool, 0)

	c := cron.New()
	c.AddFunc(config.Cron.Pattern, scheduleJobSendRemain)

	c.Start()
	defer c.Stop()

	select {
	case <-time.After(30 * time.Minute):
		sugar.Info("time out cron stop...")

	case <-cronQuit:
		sugar.Info("job finished")
	}
}

func getCookie() {

	URL := "https://ihis.gzfezx.com/fyfwh-web/public/checkRedirectInfo?index=4"

	URL = "https://open.weixin.qq.com/connect/oauth2/authorize?appid=wx3f655f24e29a6117&redirect_uri=https%3A%2F%2Fihis.gzfezx.com%2Ffyfwh-web%2Fwxfwh%2FjumpIndexPage%3Fredirect%3D%2Fpublic%2Fcategory_7.html&response_type=code&scope=snsapi_base&state=1658824968108&connect_redirect=1#wechat_redirect"

	req, _ := http.NewRequest("GET", URL, nil)

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63070517)")

	resp, _ := client.Do(req)

	respBody, _ := ioutil.ReadAll(resp.Body)
	sugar.Panicf("resp status:%v,cookie:%+v,body:%s", resp.StatusCode, resp.Cookies(), respBody)

}

// get doctor registration info
func findDoctorRegInfo() {

	if config.Display.DoctorRegistrationInfo == false {
		return
	}

	URL := "https://ihis.gzfezx.com/fyfwh-web/public/findDoctorRegInfo"

	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		sugar.Errorw("create request failure",
			"err", err.Error())
	}

	t := time.Now()

	query := req.URL.Query()
	query.Add("hospitalId", config.RegistrationInfo.HospitalID)
	query.Add("deptId", config.RegistrationInfo.DepartmentID)
	query.Add("doctorId", config.RegistrationInfo.DoctorID)
	query.Add("beginDate", t.AddDate(0, 0, 1).Format("2006-01-02"))
	query.Add("endDate", t.AddDate(0, 0, 7).Format("2006-01-02"))
	query.Add("t", strconv.FormatInt(t.UnixMilli(), 10))

	req.URL.RawQuery = query.Encode()

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		sugar.Errorw("get response failure",
			"err", err.Error())
		return
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		sugar.Errorw("read response body failure",
			"err")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		sugar.Errorw("response error",
			"statusCode", resp.StatusCode,
			"response", string(respBody))
		return
	}

	drRegInfo := new(DoctorRegInfo)

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(respBody, drRegInfo)
	if err != nil {
		sugar.Errorw("json unmarshall failure",
			"err", err.Error(),
			"json", string(respBody))
		return
	}

	sugar.Infof("docter name: %s, department name: %s", drRegInfo.DoctorName, drRegInfo.DeptName)

	for i := 0; i < len(drRegInfo.RegList); i++ {

		fee, _ := strconv.Atoi(drRegInfo.RegList[i].BabyTreatFee)
		fee = fee / 100

		regLeaveCount, _ := strconv.Atoi(drRegInfo.RegList[i].RegLeaveCount)
		regTotalCount, _ := strconv.Atoi(drRegInfo.RegList[i].RegTotalCount)

		regRemainCount := regTotalCount - regLeaveCount

		t, _ := time.Parse("2006-01-02", drRegInfo.RegList[i].RegDate)

		sugar.Infof("date: %v %v, remain: %v/%v, cost: %v", drRegInfo.RegList[i].RegDate, t.Weekday().String(), regRemainCount, regTotalCount, fee)

		if regRemainCount != 0 {
			getDoctorRegTimeInfo(drRegInfo.RegList[i].RegDate)
			time.Sleep(2 * time.Second)
		}

	}

}

func getDoctorRegTimeInfo(date string) []DoctorRegTimeInfo {

	if config.Display.DoctorRegistrationTimeInfo == false || date == "" {
		return nil
	}

	URL := "https://ihis.gzfezx.com/fyfwh-web/public/getDrRegTimeInfo001"

	param := url.Values{}
	param.Add("hospitalId", config.RegistrationInfo.HospitalID)
	param.Add("deptId", config.RegistrationInfo.DepartmentID)
	param.Add("doctorId", config.RegistrationInfo.DoctorID)
	param.Add("regDate", date)
	param.Add("scheduleType", "")
	param.Add("t", strconv.FormatInt(time.Now().UnixMilli(), 10))

	reqBody := param.Encode()

	req, err := http.NewRequest("POST", URL, strings.NewReader(reqBody))

	if err != nil {
		sugar.Errorw("create request failure",
			"err", err.Error())
		return nil
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := client.Do(req)
	if err != nil {
		sugar.Errorw("get response failure",
			"err", err.Error())
		return nil
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		sugar.Errorw("read response failure",
			"err", err.Error())
		return nil
	}

	defer resp.Body.Close()

	json := jsoniter.ConfigCompatibleWithStandardLibrary

	drRegTimeInfos := make([]DoctorRegTimeInfo, 0)

	err = json.Unmarshal(respBody, &drRegTimeInfos)
	if err != nil {
		sugar.Errorw("json unmarshall failure",
			"err", err.Error(),
			"response body", string(respBody))
		return nil
	}

	for i := 0; i < len(drRegTimeInfos); i++ {

		if drRegTimeInfos[i].RegLeaveCount == "" {
			drRegTimeInfos[i].RegLeaveCount = "0"
		}

		regLeaveCount, _ := strconv.Atoi(drRegTimeInfos[i].RegLeaveCount)
		regTotalCount, _ := strconv.Atoi(drRegTimeInfos[i].RegTotalCount)

		regRemainCount := regTotalCount - regLeaveCount

		if config.Display.DoctorValidRegistration == true && regRemainCount == 0 {
			continue
		}

		sugar.Infof("begin time: %v, end time: %v, remain: %v/%v", drRegTimeInfos[i].BeginTime, drRegTimeInfos[i].EndTime, regRemainCount, regTotalCount)
	}

	return drRegTimeInfos

}

func scheduleJobSendRemain() {
	findRegRemain := false

	if config.Cron.Schedule == false || config.Cron.Date == "" || config.Cron.BotWebHook == "" {
		return
	}

	sugar.Info("start cron...")

	configBeginTime, err := time.Parse("15:04", config.Cron.BeginTime)
	if err != nil {
		sugar.Errorw("parse config begin time error",
			"err", err.Error(),
			"config begin time", config.Cron.BeginTime)
	}

	configEndTime, err := time.Parse("15:04", config.Cron.EndTime)
	if err != nil {
		sugar.Errorw("parse config begin time error",
			"err", err.Error(),
			"config begin time", config.Cron.BeginTime)
	}

	drRegTimeInfos := getDoctorRegTimeInfo(config.Cron.Date)

	for i := 0; i < len(drRegTimeInfos); i++ {

		if drRegTimeInfos[i].RegLeaveCount == "" {
			drRegTimeInfos[i].RegLeaveCount = "0"
		}

		regLeaveCount, _ := strconv.Atoi(drRegTimeInfos[i].RegLeaveCount)
		regTotalCount, _ := strconv.Atoi(drRegTimeInfos[i].RegTotalCount)

		regRemainCount := regTotalCount - regLeaveCount

		if regRemainCount == 0 {
			continue
		}

		if config.Cron.FilterByTime == true {
			beginTime, err := time.Parse("15:04", drRegTimeInfos[i].BeginTime)
			if err != nil {
				sugar.Errorw("parse begin time failure",
					"err", err.Error(),
					"begin time", drRegTimeInfos[i].BeginTime)
			}

			endTime, err := time.Parse("15:04", drRegTimeInfos[i].EndTime)
			if err != nil {
				sugar.Errorw("parse begin time failure",
					"err", err.Error(),
					"end time", drRegTimeInfos[i].EndTime)
			}

			if beginTime.Before(configBeginTime) || endTime.After(configEndTime) {
				continue
			}

		}

		message := fmt.Sprintf("date: %v, begin time: %v, end time: %v, remain: %v/%v", config.Cron.Date, drRegTimeInfos[i].BeginTime, drRegTimeInfos[i].EndTime, regRemainCount, regTotalCount)

		sendWeatherContentTextToWeworkBot(message)

		findRegRemain = true
	}

	if findRegRemain == true {
		sendWeatherContentTextToWeworkBot("cron stop...")
		cronQuit <- true
	} else {
		sugar.Info("no remain")
	}
}

func sendWeatherContentTextToWeworkBot(text string) {

	bc := new(WeworkBotContent)
	bc.Msgtype = "text"
	bc.Text.Content = text

	reqBody, err := jsoniter.MarshalToString(bc)
	if err != nil {
		sugar.Errorw("marshal text failure",
			"err", err.Error())
	}

	client := http.Client{}

	url := `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=` + config.Cron.BotWebHook

	req, _ := http.NewRequest("POST", url, strings.NewReader(reqBody))

	client.Do(req)

}
