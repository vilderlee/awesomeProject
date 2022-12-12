package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron/v3"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const wxURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=76ee6713-74c9-4e07-a483-91ad8a5a95f3"
const standform = "今日运维值班人员<font color=\\\"warning\\\">列表</font>，请相关同事注意。\n     " +
	"    >水险:<font color=\\\"comment\\\">%s</font>\n       " +
	"  >财产险:<font color=\\\"comment\\\">%s</font>\n"

const contentType = "application/json;charset=utf-8"

// 每个人一个礼拜，如果全部值班结束重新开始
func main() {

	// 新建一个定时任务对象
	// 根据cron表达式进行时间调度，cron可以精确到秒，大部分表达式格式也是从秒开始。
	//crontab := cron.New()  默认从分开始进行时间调度
	crontab := cron.New(cron.WithSeconds()) //精确到秒
	//定时任务
	spec := "*/60 * * * * ?" //cron表达式，每五秒一次
	// 添加定时任务,
	crontab.AddFunc(spec, task)
	// 启动定时器
	crontab.Start()
	// 定时任务是另起协程执行的,这里使用 select 简答阻塞.实际开发中需要
	// 根据实际情况进行控制
	select {} //阻塞主线程停止
}

func task() {
	now := time.Now()

	if !isWorkingDay(now) {
		return
	}

	insurance := make([]string, 0)
	insurance = append(insurance, "marineInsurance.txt")
	insurance = append(insurance, "propertyInsurance.txt")

	personList := make([]string, 0)
	for _, fileName := range insurance {

		propertyInsurancePeople := getPeople(fileName)
		isReset := true

		for _, person := range propertyInsurancePeople {

			isReset = isReset && person.isWorked
			// 工作日正在工作，则通知
			if person.isWorking {
				personList = append(personList, person.name)
				// 周五的话就更新状态
				if isFriDay(now) {
					person.isWorking = false
					person.isWorked = false
					updatePerson(fileName, propertyInsurancePeople)
				}
				break
			}
			// 还未值班过
			if !person.isWorked {
				personList = append(personList, person.name)
				person.isWorked = true
				person.isWorking = true
				updatePerson(fileName, propertyInsurancePeople)
				break
			}
		}

		if isReset {
			reset(fileName, propertyInsurancePeople)
		}
	}

	outString := fmt.Sprintf(standform, personList[0], personList[1])
	fmt.Println(outString)

	message := &Message{Msgtype: "markdown", Markdown: &Text{
		Content:        outString,
		Mentioned_list: []string{"@all"},
	}}

	jsonBytes, _ := json.Marshal(message)
	fmt.Println(string(jsonBytes))

	resp, err := http.Post(wxURL, contentType, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Fatal(err)

	}
	fmt.Println(resp)
}

func isWorkingDay(now time.Time) bool {
	//return now.Weekday() != time.Sunday &&
	//	now.Weekday() != time.Saturday
	return true
}

func isFriDay(now time.Time) bool {
	return now.Weekday() == time.Friday
}

func updatePerson(name string, people []*Person) {
	flushFile(name, people)
}

func reset(name string, people []*Person) {
	for _, person := range people {
		person.isWorked = false
		person.isWorking = false
	}
	flushFile(name, people)
}

func getPeople(name string) []*Person {
	fs, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}

	bytes, err := ioutil.ReadAll(fs)
	results := strings.Split(string(bytes), "\n")
	persons := make([]*Person, 0)
	for _, result := range results {
		res := strings.Split(result, ",")
		isWorking, _ := strconv.ParseBool(res[1])
		isSkip, _ := strconv.ParseBool(res[2])
		persons = append(persons, &Person{res[0], isWorking, isSkip})
	}

	return persons
}

func flushFile(name string, people []*Person) {
	fs, err := os.Open(name)
	if err != nil {
		log.Print("cannot find propertyInsurance.txt file")
		log.Fatal(err)
	}
	for _, person := range people {
		str := person.name + "," + strconv.FormatBool(person.isWorking) + "," + strconv.FormatBool(person.isWorked)
		_, err = fs.WriteString(str)
		if err != nil {
			return
		}
	}
}

type Person struct {
	name      string
	isWorking bool
	isWorked  bool
}

type Message struct {
	Msgtype  string `json:"msgtype"`
	Markdown *Text  `json:"markdown"`
}

type Text struct {
	Content        string   `json:"content"`
	Mentioned_list []string `json:"mentioned_list"`
}
