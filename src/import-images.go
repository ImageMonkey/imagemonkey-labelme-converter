package main

import (
	"fmt"
	"os"
	"bufio"
	"strings"
)

//change me accordingly

const LABEL = "car"
const ACTION = "push"
const PRODUCTION = false
const AUTO_UNLOCK = false

func showWarningAndContinue(num int) bool {
	if PRODUCTION {
		fmt.Printf("PRODUCTION SAFETY CHECK\n\n#Images: %d\nLabel: %s\nauto unlock: %t\n\nAre you sure you want to do this? [yes/no]\n", num, LABEL, AUTO_UNLOCK)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.Trim(input, "\r\n")
		if input == "yes" {
			return true
		}
	} else {
		return true
	}

	return false
}


func main() {
	apiBaseUrl := "http://127.0.0.1:8081"
	if PRODUCTION {
		apiBaseUrl = "" //TODO
	}

	imageMonkeyAPI := NewImageMonkeyAPI(apiBaseUrl)
	labelMeDataset := NewLabelMeDataset("D:\\dataset", true)
	//labelMeDataset := NewLabelMeDataset("../dataset", true)
	labelMeDataset.Load()
	imageInfos, err := labelMeDataset.GetImageInfos(LABEL)
	if err != nil {
		fmt.Printf("Couldn't get image infos: %s", err.Error())
		return
	}

	if ACTION == "download" {
		err = labelMeDataset.DownloadImages(imageInfos, "car")
		if err != nil {
			fmt.Printf("Couldn't download image: %s", err.Error())
		}
	} else if ACTION == "push" {
		if !showWarningAndContinue(len(imageInfos)) {
			fmt.Printf("aborted\n")
			return
		}

		for _, elem := range imageInfos {
			img, err := labelMeDataset.GetImage(LABEL, elem, true)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			err = imageMonkeyAPI.AddLabelMeDonation(img, LABEL, AUTO_UNLOCK)
			if err != nil {
				fmt.Println(err.Error())
				//return
			}

			fmt.Printf("Added image: %s\n", elem.UniqueName)
			//return
		}

	} else {
		fmt.Printf("Invalid action: %s", ACTION)
		return
	}
}