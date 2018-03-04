package main

import (
	"fmt"
	"os/exec"
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"encoding/xml"
	"encoding/json"
	"io/ioutil"
	"errors"
	"io"
	"net/http"
)

type Object struct {
	XMLName xml.Name `xml:"object"`
	Name string `xml:"name"`
}

type Annotation struct {
	XMLName xml.Name `xml:"annotation"`
	Objects []Object `xml:"object"`
	Filename string `xml:"filename"`
	Folder string `xml:"folder"`
}

type Label struct {
	Name string `json:"name"`
	Num int32 `json:"num"`
}

type ImageInfo struct {
	Folder string `json:"folder"`
	Filename string `json:"filename"`
}

func getXmlFilesFromDir(dir string) ([]string, error) {
	fileList := []string{}
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".xml") {
			fileList = append(fileList, path)
		}
        return nil
	})

	return fileList, err
}

func readCachedLabelMap(path string) (map[string]int32, error) {
	labelMap := make(map[string]int32)

	bytes, err := ioutil.ReadFile(path)
    if err != nil {
        fmt.Println(err.Error())
        return labelMap, err
    }

    var labels []Label
    err = json.Unmarshal(bytes, &labels)
    if err != nil {
    	fmt.Println(err.Error())
    	return labelMap, err
    }

    for _, label := range labels {
    	labelMap[label.Name] = label.Num
    }

    return labelMap, nil
}


func readCachedImageInfos(path string) ([]ImageInfo, error) {
	var imageInfos []ImageInfo

	bytes, err := ioutil.ReadFile(path)
    if err != nil {
        fmt.Println(err.Error())
        return imageInfos, err
    }

    
    err = json.Unmarshal(bytes, &imageInfos)
    if err != nil {
    	fmt.Println(err.Error())
    	return imageInfos, err
    }

    return imageInfos, nil
}

func persistLabelMap(path string, labelMap map[string]int32) error {
	var labels []Label
	for k, v := range labelMap {
		var label Label
		label.Name = k
		label.Num = v
		labels = append(labels, label)
	}

	bytes, err := json.Marshal(labels)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

func persistImageInfos(path string, imageInfos []ImageInfo) error {
	bytes, err := json.Marshal(imageInfos)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}



type Dataset interface {
    Load() error
    BuildLabelMap(outputFolder string) error
    GetLabelMap() map[string]int32
}

type LabelMeDataset struct {
	baseDirectory string
	labels map[string]int32
	baseUrl string
	useCache bool
}

func NewLabelMeDataset(useCache bool) *LabelMeDataset {
    return &LabelMeDataset{
    	labels: make(map[string]int32),
    	baseUrl: "http://people.csail.mit.edu/brussell/research/LabelMe/",
    	useCache: useCache,
    } 
}

func (p *LabelMeDataset) Load(outputFolder string) error {
	p.baseDirectory = outputFolder

	if _, err := os.Stat(outputFolder); os.IsNotExist(err) {
		fmt.Printf("dataset doesn't exist...downloading\n")
		err = os.Mkdir(outputFolder, os.ModeDir)
		if err != nil {
			fmt.Printf("Couldn't create folder: ", outputFolder)
			return errors.New(("Couldn't create folder " + outputFolder))
		}
		//download labelme dataset
		cmd := exec.Command("wget", "-m", "-np", (p.baseUrl + "Annotations/"), "--directory-prefix=" + outputFolder)
		stderr, _ := cmd.StderrPipe()
	    cmd.Start()

	    scanner := bufio.NewScanner(stderr)
	    scanner.Split(bufio.ScanWords)
	    for scanner.Scan() {
	        m := scanner.Text()
	        fmt.Println(m)
	    }
	    cmd.Wait()



	} else {
		fmt.Println("dataset already exists...using this one")
	}

    return nil
}

func (p *LabelMeDataset) BuildLabelMap() error {
	cachedLabelsMapDir := p.baseDirectory + "/cache/"
	cachedLabelsMapPath := cachedLabelsMapDir + "labels.map"

	//if cache is enabled
	if p.useCache {
		if _, err := os.Stat(cachedLabelsMapDir); os.IsNotExist(err) {
			err := os.Mkdir(cachedLabelsMapDir, os.ModeDir)
			if err != nil {
				return err
			}
		}


		//check if labels map file exists
		if _, err := os.Stat(cachedLabelsMapPath); err == nil {
			//if file exists..read it and we are done here.
			fmt.Println("found cached label map...using this one")
			p.labels, err = readCachedLabelMap(cachedLabelsMapPath)
			return err
		}
	}

	files, err := getXmlFilesFromDir(p.baseDirectory)
	if err != nil {
		return err
	}


	//parse annotation from xml file
	for _, file := range files {
        annotation, err := p.ParseAnnotationFromXml(file)
        if err != nil {
        	//looks like there are some broken XML files in the label me dataset...skip those 
        	fmt.Println("Couldn't parse xml file")
        	continue
        }

        for _, object := range annotation.Objects {
        	if val, ok := p.labels[object.Name]; ok { //already contains
        		p.labels[object.Name] = (val + 1)
        	} else {
        		p.labels[object.Name] = 1
        	}
        }
	}

	if p.useCache {
		err = persistLabelMap(cachedLabelsMapPath, p.labels)
		return err
	}

	return nil
}

func (p *LabelMeDataset) GetLabelMap() map[string]int32 {
	return p.labels
}

func (p *LabelMeDataset) GetImageInfos(label string) ([]ImageInfo, error) {
	cachedImageInfosDir := p.baseDirectory + "/cache/"
	cachedImageInfos := cachedImageInfosDir + label + ".tmp"

	var imageInfos []ImageInfo

	if p.useCache {
		if _, err := os.Stat(cachedImageInfosDir); os.IsNotExist(err) {
			err := os.Mkdir(cachedImageInfosDir, os.ModeDir)
			if err != nil {
				return imageInfos, err
			}
		}

		//check if file exists
		if _, err := os.Stat(cachedImageInfos); err == nil {
			//if file exists..read it and we are done here.
			fmt.Println("found cached image infos...using this one")
			imageInfos, err = readCachedImageInfos(cachedImageInfos)
			return imageInfos, err
		}

	}

	filenameExistsMap := map[string]bool{}

	files, err := getXmlFilesFromDir(p.baseDirectory)
	for _, file := range files {
		annotation, err := p.ParseAnnotationFromXml(file)
		if err != nil {
			//looks like there are some broken XML files in the label me dataset...skip those 
        	fmt.Println("Couldn't parse xml file")
        	continue
		}

		found := false
		for _, object := range annotation.Objects {
			if object.Name == label {
				found = true
				break
			}
		}


		if found {
			var imageInfo ImageInfo
			imageInfo.Filename = annotation.Filename
			imageInfo.Folder = annotation.Folder

			fullname := annotation.Folder + "/" + annotation.Filename
			_, exists := filenameExistsMap[fullname]
			if !exists {
				imageInfos = append(imageInfos, imageInfo)
				filenameExistsMap[fullname] = true
			}
		}
	}

	if p.useCache {
		err = persistImageInfos(cachedImageInfos, imageInfos)
		return imageInfos, err
	}

	return imageInfos, nil
} 

func (p *LabelMeDataset) DownloadImage(name string, filename string) (error) {
	url := p.baseUrl + "Images/" + name
    response, err := http.Get(url)
    if err != nil {
        return err
    }

    defer response.Body.Close()

    file, err := os.Create(filename)
    if err != nil {
        return err
    }

    _, err = io.Copy(file, response.Body)
    if err != nil {
        return err
    }
    file.Close()

    return nil
}

func (p *LabelMeDataset) DownloadImages(imageInfos []ImageInfo, label string) (error) {
	dir := p.baseDirectory + "/cache/" + label
	if p.useCache {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.Mkdir(dir, os.ModeDir)
			if err != nil {
				return err
			}
		} else {
			fmt.Println("images folder already exists...using this one")
			return nil
		}
	}

	for _, imageInfo := range imageInfos {
		err := p.DownloadImage((imageInfo.Folder + "/" + imageInfo.Filename), (dir + "/" + imageInfo.Folder + "_" + imageInfo.Filename))
		if err != nil {
			return err
		}
	}

	return nil
}


func (p *LabelMeDataset) ParseAnnotationFromXml(filename string) (Annotation, error) {
	var annotation Annotation
	f, err := os.Open(filename)
	if err != nil {
		return annotation, err
	}
	defer f.Close()

	
	byteValue, _ := ioutil.ReadAll(f)
	err = xml.Unmarshal(byteValue, &annotation)
	if err != nil {
		return annotation, err
	}

	return annotation, nil
}

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