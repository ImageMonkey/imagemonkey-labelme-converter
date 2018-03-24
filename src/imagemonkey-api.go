package main

import (
    "fmt"
    "mime/multipart"
    "net/http"
    "bytes"
    "encoding/json"
    "image/jpeg"
    "io/ioutil"
    "errors"
)

func bool2string(in bool) string {
    if in {
        return "true"
    }
    return "false"
}

type PolyPoint struct {
    X int32 `json:"x"`
    Y int32 `json:"y"`
}

type ImageMonkeyPolygonAnnotation struct {
    Points []PolyPoint `json:"points"`
    Angle int32 `json:"angle"`
    Type string `json:"type"`
}


type ImageMonkeyAnnotation struct {
    Annotations []ImageMonkeyPolygonAnnotation `json:"annotations"`
    Label string `json:"label"`
}

type ImageMonkeyAPI struct {
	baseUrl string
}

func NewImageMonkeyAPI(baseUrl string) *ImageMonkeyAPI {
    return &ImageMonkeyAPI {
        baseUrl: baseUrl,
    } 
}

func (p *ImageMonkeyAPI) AddAnnotations(imageId string, annotation ImageMonkeyAnnotation) error {
    url := p.baseUrl + "/v1/annotate/" + imageId


    jsonStr, err := json.Marshal(annotation)
    if err != nil {
        return err
    }

    fmt.Println(string(jsonStr))

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
    //req.Header.Set("X-Custom-Header", "myvalue")
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    fmt.Println("response Status:", resp.Status)
    fmt.Println("response Headers:", resp.Header)
    body, _ := ioutil.ReadAll(resp.Body)
    fmt.Println("response Body:", string(body))

    return nil
}



func _donate(img Image, provider string, baseUrl string, label string, autoUnlock bool) error {
    var b bytes.Buffer
    w := multipart.NewWriter(&b)

    url := ""
    if provider == "donation" {
        url = baseUrl + "/v1/donate"
    } else if provider == "labelme" {
        url = baseUrl + "/v1/internal/labelme/donate"
    } else {
        err := errors.New(("Invalid provider: " + provider))
        return err
    }

    fw, err := w.CreateFormFile("image", "test.jpeg")
    if err != nil {
        return err
    }


    buf := new(bytes.Buffer)
    err = jpeg.Encode(buf, img.ScaledImage, nil)
    
    _, err = fw.Write(buf.Bytes())
    if err != nil {
        return err
    }

    fw, err = w.CreateFormField("label")
    if err != nil {
        return err
    }
    _, err = fw.Write([]byte(label))
    if err != nil {
        return err
    }


    autoUnlockStr := bool2string(autoUnlock)

    fw, err = w.CreateFormField("auto_unlock")
    if err != nil {
        return err
    }
    _, err = fw.Write([]byte(autoUnlockStr))
    if err != nil {
        return err
    }

    if provider == "labelme" {
        fw, err = w.CreateFormField("image_source_url")
        if err != nil {
            return err
        }
        _, err = fw.Write([]byte(img.Url))
        if err != nil {
            return err
        }
    }

    // Don't forget to close the multipart writer.
    // If you don't close it, your request will be missing the terminating boundary.
    w.Close()

    // Now that you have a form, you can submit it to your handler.
    req, err := http.NewRequest("POST", url, &b)
    if err != nil {
        return err
    }

    if provider == "labelme" {
        req.Header.Set("X-Client-Secret", X_CLIENT_SECRET)
        req.Header.Set("X-Client-Id", X_CLIENT_ID)
    }

    // Don't forget to set the content type, this will contain the boundary.
    req.Header.Set("Content-Type", w.FormDataContentType())

    // Submit the request
    client := &http.Client{}
    res, err := client.Do(req)
    if err != nil {
        return err
    }

    // Check the response
    if res.StatusCode != http.StatusOK {
        //var data map[string]interface{}
        body, err := ioutil.ReadAll(res.Body)
        if err != nil {
            return err
        }

        /*err = json.Unmarshal(body, &data)
        if err != nil {
            return err
        }*/

        return errors.New(string(body))
    }
    return nil
}

func (p *ImageMonkeyAPI) Donate(img Image, label string) error {
    return _donate(img, "donation", p.baseUrl, label, false)
}

func (p *ImageMonkeyAPI) AddLabelMeDonation (img Image, label string, autoUnlock bool) error {
    return _donate(img, "labelme", p.baseUrl, label, autoUnlock)
}

func (p *ImageMonkeyAPI) ConvertFrom(label string, annotation Annotation, scaleFactor float32) ImageMonkeyAnnotation {
    var imagemonkeyAnnotations []ImageMonkeyPolygonAnnotation
    for _, object := range annotation.Objects {

        var imagemonkeyAnnotation ImageMonkeyPolygonAnnotation
        imagemonkeyAnnotation.Type = "polygon"
        imagemonkeyAnnotation.Angle = 0
        imagemonkeyAnnotation.Points = make([]PolyPoint, 0) //empty slice

        for _, point := range object.Polygon.Points {
            var imagemonkeyPoint PolyPoint
            imagemonkeyPoint.X = int32(float32(point.X) * scaleFactor)
            imagemonkeyPoint.Y= int32(float32(point.Y) * scaleFactor)

            imagemonkeyAnnotation.Points = append(imagemonkeyAnnotation.Points, imagemonkeyPoint)
        }

        imagemonkeyAnnotations = append(imagemonkeyAnnotations, imagemonkeyAnnotation)
    }

    var anno ImageMonkeyAnnotation
    anno.Annotations = imagemonkeyAnnotations
    anno.Label = label

    return anno
}