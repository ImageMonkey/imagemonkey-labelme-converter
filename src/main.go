package main

import (
	"fmt"
)

func main() {
	outputFolder := "../dataset"

	labelMeDataset := NewLabelMeDataset(true)
	labelMeDataset.Load(outputFolder)
	labelMeDataset.BuildLabelMap()
	_ = labelMeDataset.GetLabelMap()

	imageInfos, err := labelMeDataset.GetImageInfos("car")
	if err != nil {
		fmt.Println("Couldn't get image infos: ", err.Error())
		return
	}

	err = labelMeDataset.DownloadImages(imageInfos, "car")
	if err != nil {
		fmt.Println("Couldn't download image: ", err.Error())
		return	
	}
	
}