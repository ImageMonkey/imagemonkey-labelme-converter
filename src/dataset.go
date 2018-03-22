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
	_"image/jpeg"
	_"image/png"
	"image"
	"github.com/nfnt/resize"
)

type Box struct {
	XMLName xml.Name `xml:"box"`
	Xmin float32 `xml:"xmin"`
	Ymin float32 `xml:"ymin"`
	Xmax float32 `xml:"xmax"`
	Ymax float32 `xml:"ymax"`
}

type Segment struct {
	XMLName xml.Name `xml:"segm"`
	Box Box `xml:"box,omitempty"`
}

type Point struct {
	XMLName xml.Name `xml:"pt"`
	X int32 `xml:"x"`
	Y int32 `xml:"y"`
}

type Polygon struct {
	XMLName xml.Name `xml:"polygon"`
	Points []Point `xml:"pt"`
}

type Object struct {
	XMLName xml.Name `xml:"object"`
	Name string `xml:"name"`
	Polygon Polygon `xml:"polygon,omitempty"`
	Segment Segment `xml:"segm,omitempty"`
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
	UniqueName string `json:"uniquename"`
}

type Image struct {
	OriginalImage image.Image `json:"original_image"`
	ScaledImage image.Image `json:"scaled_image"`
	OriginalWidth int32 `json:"original_width"`
	OriginalHeight int32 `json:"original_height"`
	ScaledWidth int32 `json:"scaled_width"`
	ScaledHeight int32 `json:"scaled_height"`
	ScaleFactor float32 `json:"scalefactor"`
	Url string `json:"url"`
}

func calcScaleFactor(img Image) float32 {
	var maxSize int32
	var scaleFactor float32

	maxSize = 1000
	scaleFactor = 1.0

	if img.OriginalWidth > img.OriginalHeight {
		if img.OriginalWidth > maxSize {
			scaleFactor = float32(maxSize)/float32(img.OriginalWidth)
		} else {
			scaleFactor = 1.0
		}
	} else {
		if img.OriginalHeight > maxSize {
			scaleFactor = float32(maxSize)/float32(img.OriginalHeight)
		} else {
			scaleFactor = 1.0
		}
	}

	return scaleFactor
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

func convertToLocalFilename(folder string, filename string) string {
	return folder + "_" + filename
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

func NewLabelMeDataset(baseDirectory string, useCache bool) *LabelMeDataset {
    return &LabelMeDataset{
    	labels: make(map[string]int32),
    	baseUrl: "http://people.csail.mit.edu/brussell/research/LabelMe/",
    	useCache: useCache,
    	baseDirectory: baseDirectory,
    } 
}

func (p *LabelMeDataset) Load() error {
	if _, err := os.Stat(p.baseDirectory); os.IsNotExist(err) {
		fmt.Printf("dataset doesn't exist...downloading\n")
		err = os.Mkdir(p.baseDirectory, os.ModeDir)
		if err != nil {
			fmt.Printf("Couldn't create folder: ", p.baseDirectory)
			return errors.New(("Couldn't create folder " + p.baseDirectory))
		}
		//download labelme dataset
		cmd := exec.Command("wget", "-m", "-np", (p.baseUrl + "Annotations/"), "--directory-prefix=" + p.baseDirectory)
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

func (p *LabelMeDataset) GetCacheDirectory() string {
	return p.baseDirectory + "/cache/"
}

func (p *LabelMeDataset) BuildLabelMap() error {
	cachedLabelsMapDir := p.GetCacheDirectory()
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
        annotation, err := p.ParseAnnotationFromXml(file, "")
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
	cachedImageInfosDir := p.GetCacheDirectory()
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
		annotation, err := p.ParseAnnotationFromXml(file, "")
		if err != nil {
			//looks like there are some broken XML files in the label me dataset...skip those 
        	fmt.Println("Couldn't parse xml file %s", err.Error())
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
			//trim any newline characters in the folder/file name (for some reason, there are some
			//files in the labelme dataset that have newline chars?)
			imageInfo.Filename = strings.Trim(annotation.Filename, "\r\n")
			imageInfo.Folder = strings.Trim(annotation.Folder, "\r\n")
			imageInfo.UniqueName = convertToLocalFilename(imageInfo.Folder, imageInfo.Filename)

			fullname := imageInfo.Folder + "/" + imageInfo.Filename
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
	dir := p.GetCacheDirectory() + label
	if p.useCache {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.Mkdir(dir, os.ModeDir)
			if err != nil {
				return err
			}
		} else {
			fmt.Println("images folder already exists...using this one")
			//return nil
		}
	}

	lastDownloadedImage := ""
	for i, imageInfo := range imageInfos {
		path := dir + "/" + convertToLocalFilename(imageInfo.Folder, imageInfo.Filename)
		if _, err := os.Stat(path); err == nil { //skip files that already exists
			fmt.Printf("[%d/%d] Image exists, skipping: %s\n", i+1, len(imageInfos), convertToLocalFilename(imageInfo.Folder, imageInfo.Filename))
			lastDownloadedImage = imageInfo.Folder + "/" + imageInfo.Filename
			continue
		}

		if lastDownloadedImage != "" {
			//remove the image we have download last, as it might be that the image is broken due to the fact, that we interrupted the download process
			//by pressing Ctrl+C. By re-downloading the image we are assuring, that this won't happen. 
			os.Remove(lastDownloadedImage)

			err := p.DownloadImage((imageInfo.Folder + "/" + imageInfo.Filename), path)
			if err != nil {
				return err
			}
			fmt.Printf("[%d/%d] Re-Downloaded Image %s\n", i+1, len(imageInfos), convertToLocalFilename(imageInfo.Folder, imageInfo.Filename))
			lastDownloadedImage = ""
		}

		err := p.DownloadImage((imageInfo.Folder + "/" + imageInfo.Filename), path)
		if err != nil {
			return err
		}
		fmt.Printf("[%d/%d] Downloaded Image %s\n", i+1, len(imageInfos), convertToLocalFilename(imageInfo.Folder, imageInfo.Filename))
	}

	return nil
}

func (p *LabelMeDataset) GetImage(label string, imageInfo ImageInfo, scaled bool) (Image, error) {
	var im Image
	if p.useCache {
		cacheDir := p.GetCacheDirectory()
		f, err := os.Open(cacheDir + label + "/" + imageInfo.UniqueName)
	    if err != nil {
	        return im, err
	    }
	    defer f.Close()


	    im.OriginalImage, _, err = image.Decode(f)
	    if err != nil {
	        return im, err
	    }

	    bounds := im.OriginalImage.Bounds()
	    im.OriginalWidth = int32(bounds.Dx())
    	im.OriginalHeight = int32(bounds.Dy())

    	if(scaled){
    		im.ScaleFactor = calcScaleFactor(im)
    		im.ScaledWidth = int32(float32(im.OriginalWidth) * im.ScaleFactor)
    		im.ScaledHeight = int32(float32(im.OriginalHeight) * im.ScaleFactor)

    		im.ScaledImage = resize.Resize(uint(im.ScaledWidth), uint(im.ScaledHeight), im.OriginalImage, resize.Lanczos3)

    	} else {
    		im.ScaleFactor = 1.0
    		im.ScaledWidth = im.OriginalWidth
    		im.ScaledHeight = im.OriginalHeight
    		im.ScaledImage = im.OriginalImage
    	}

    	im.Url = p.baseUrl + "Images/" + imageInfo.Folder + "/" + imageInfo.Filename

		return im, nil
	}

	return im, errors.New("LabelMeConverter: Currently only the cached version of this call is implemented")
}


func (p *LabelMeDataset) ParseAnnotationFromXml(filename string, label string) (Annotation, error) {
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

	if label != "" { //only keep objects where label matches
		var objects []Object
		objects = make([]Object, 0) //empty slice 
		for _, object := range annotation.Objects {
			if label == object.Name {
				continue
			}

			objects = append(objects, object)
		}
		annotation.Objects = objects
	}

	return annotation, nil
}
