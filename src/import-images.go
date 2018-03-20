package main

import (
	"fmt"
)

//change me accordingly

const LABEL = "car"
const ACTION = "download"
const PRODUCTION = false




func main() {
	apiBaseUrl := "http://127.0.0.1:8081"
	if PRODUCTION {
		apiBaseUrl = "" //TODO
	}

	imageMonkeyAPI := NewImageMonkeyAPI(apiBaseUrl)
	labelMeDataset := NewLabelMeDataset("../dataset", true)
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
		for _, elem := range imageInfos {
			img, err := labelMeDataset.GetImage(LABEL, elem, true)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			err = imageMonkeyAPI.AddLabelMeDonation(img)
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