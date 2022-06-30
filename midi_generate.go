package midi

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Data struct {
	time       float64
	freq       int
	confidence float64
}

type lrcLine struct {
	beginTime float64
	endTime   float64
	freq      int
}

const (
	MinPitchLength      = 0.2
	ContinuesInValidThr = 3
)

func IsPitchValid(frequency int, confidence float64) bool {
	if frequency < 100 || frequency > 650 {
		return false
	}
	if confidence < 0.6 {
		return false
	}
	return true
}

func Decimal(value float64) float64 {
	value, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return value
}

func RemoveFile(filePath string) {
	deleteArgs := []string{filePath}
	command := exec.Command("rm", deleteArgs...)
	err := command.Run()
	if err != nil {
		fmt.Println("can't delete ", filePath)
	}
}

/*
参数。           类型         参数说明
mp3FileName    String  输入需要转歌谱的mp3文件
targetFileName String  生成歌谱的输出文件
lrcFileName    String  歌词文件(确定音乐开始时间)

*/

func GenerateMidiFile(mp3FileName string, targetFileName string, lrcFileName string) bool {
	//解析歌词开始时间
	lrcStartTime := 0
	fiLrc, errLrc := os.Open(lrcFileName)
	defer fiLrc.Close()
	if errLrc != nil {
		fmt.Println("open lrc file fail", lrcFileName)
	}
	content, _ := ioutil.ReadAll(fiLrc)
	ctxList := strings.Split(string(content), "\n")
	for _, line := range ctxList {
		var pStr string
		if strings.Contains(line, ":") && strings.Contains(line, ".") {
			pStr = strings.Trim(line, "[")
			timeLines := strings.Split(pStr, ":")
			if len(timeLines) <= 0 {
				break
			} else {
				if timeLines[0] == "00" {
					x := strings.Split(line, "]")
					if len(x) > 0 && x[0][0] == '[' {
						time := x[0][1:]
						minuteList := strings.Split(time, ":")
						if len(minuteList) <= 1 {
							continue
						}

						if minuteList[0] != "00" {
							continue
						}

						minute, _ := strconv.Atoi(minuteList[0])

						secLists := strings.Split(minuteList[1], ".")
						if len(secLists) <= 1 {
							continue
						}
						second, _ := strconv.Atoi(secLists[0])
						millSecond, _ := strconv.Atoi(secLists[1])
						lrcStartTime = (minute*60+second)*1000 + millSecond
						fmt.Println("minute:", minute, " second:", second, " millSecond", millSecond, " lrcStartTime", lrcStartTime)
						break
					}
				}
			}
		}
	}
	//转mp3为wav
	wavFileName := strings.Replace(mp3FileName, "mp3", "wav", -1)
	defer RemoveFile(wavFileName)
	ffmpegArgs := []string{"-i", mp3FileName, "-f", "wav", "-y", wavFileName}
	command := exec.Command("ffmpeg", ffmpegArgs...)
	err := command.Run()
	if err != nil {
		fmt.Println("convert mp3 to wav fail")
		return false
	}

	//利用crepe生成初步的音调文件，步长为50ms。
	crepeArgs := []string{wavFileName, "--step-size", "50"}
	commandCrepe := exec.Command("crepe", crepeArgs...)
	err = commandCrepe.Run()
	if err != nil {
		fmt.Println("crepe fail", err)
		return false
	}

	arrString := strings.Split(mp3FileName, "/")
	LEN := len(arrString)
	crepeName := strings.Replace(arrString[LEN-1]+".f0.csv", "mp3.", "", -1)
	mp3Name := arrString[LEN-1]
	crepeFilePath := strings.Replace(mp3FileName, mp3Name, crepeName, -1)
	_, err1 := os.Stat(crepeFilePath)

	if err1 != nil {
		fmt.Println("crepe file " + crepeFilePath + " not exist")
		return false
	} else {
		defer RemoveFile(crepeFilePath)
		fmt.Println("successful generate ", crepeFilePath)
	}
	fi, err := os.Open(crepeFilePath)
	if err != nil {
		fmt.Println("open file fail")
	}
	r := bufio.NewReader(fi)

	i := 0
	data := make([]Data, 0)

	//解析音调文件，存到data中
	for {
		lineBytes, Err := r.ReadBytes('\n')
		line := strings.TrimSpace(string(lineBytes))
		if Err != nil && Err != io.EOF {
			fmt.Println("read fail")
			return false
		}
		if Err == io.EOF {
			break
		}
		if i == 0 {
			i = 1
		} else {
			e := strings.Split(line, ",")
			time, _ := strconv.ParseFloat(e[0], 32)
			if time*1000 < float64(lrcStartTime)-1000 {
				continue
			}
			freq, _ := strconv.ParseFloat(e[1], 32)
			confidence, _ := strconv.ParseFloat(e[2], 32)
			if e[2] == "nan" {
				data = append(data, Data{
					time:       time,
					freq:       int(freq),
					confidence: 0,
				})
			} else {
				data = append(data, Data{
					time:       time,
					freq:       int(freq),
					confidence: confidence,
				})
			}
		}
	}
	//生成算法，生成固定长度的波形
	startTime := float64(-1)
	var pitchResult = make([]*lrcLine, 0)
	AvgFrequency := 0.0
	ContinuesInValidCnt := 0
	//item[0] 时间
	//item[1] 评率
	//item[2] 置信度
	for _, item := range data {
		if !IsPitchValid(item.freq, item.confidence) {
			//startTime != -1说明已经生成段的过程中
			if startTime != -1 {
				ContinuesInValidCnt++
				if ContinuesInValidCnt == ContinuesInValidThr {
					if item.time-ContinuesInValidThr*0.05-startTime >= MinPitchLength {
						pitchResult = append(pitchResult, &lrcLine{
							beginTime: startTime,
							endTime:   item.time - ContinuesInValidThr*0.05,
							freq:      int(AvgFrequency),
						})
						startTime = -1
						AvgFrequency = 0
						ContinuesInValidCnt = 0
					} else {
						LEN = len(pitchResult)
						if LEN > 0 && math.Abs(pitchResult[LEN-1].endTime-startTime) < 0.01 {
							pitchResult[LEN-1].endTime = item.time - ContinuesInValidThr*0.05
						}
						startTime = -1
						AvgFrequency = 0
						ContinuesInValidCnt = 0
					}
				}
			}
		} else {
			ContinuesInValidCnt = 0
			if startTime == -1 {
				startTime = item.time
				AvgFrequency = float64(item.freq)
			} else {
				if math.Abs(AvgFrequency-float64(item.freq)) >= 20 && item.time-startTime >= MinPitchLength+0.1 {
					pitchResult = append(pitchResult, &lrcLine{
						beginTime: startTime,
						endTime:   item.time,
						freq:      int(AvgFrequency),
					})
					startTime = item.time
					AvgFrequency = float64(item.freq)
					ContinuesInValidCnt = 0
				} else {
					cnt := (item.time - startTime) / 0.05
					AvgFrequency = (AvgFrequency*cnt + float64(item.freq)) / (cnt + 1)
				}
			}
		}
	}
	outputString := ""
	//fmt.Println(len(pitchResult))
	for _, item := range pitchResult {
		beginTimeString := strconv.Itoa(int(Decimal(item.beginTime) * 1000))
		endTimeString := strconv.Itoa(int(Decimal(item.endTime) * 1000))
		freqString := strconv.Itoa(item.freq)
		outputString += beginTimeString + " " + endTimeString + " " + freqString + "\n"
		//fmt.Println(beginTimeString, " ", endTimeString, " ", freqString)
	}

	f1, err2 := os.OpenFile(targetFileName, os.O_WRONLY|os.O_CREATE, 0666)
	if err2 != nil {
		fmt.Println("open file fail", err2)
		return false
	}
	defer func() {
		err = f1.Close()
		if err != nil {
			fmt.Println("关闭文件错误", err)
		}
	}()

	_, err3 := io.WriteString(f1, outputString)
	if err3 != nil {
		fmt.Println("write fail", err3)
		return false
	}
	return true
}
