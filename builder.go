package main

import (
	"bufio"
	"bytes"
	"image"
	"path"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/tidusant/c3m-common/c3mcommon"
	"github.com/tidusant/c3m-common/log"
	"github.com/tidusant/c3m-common/lzjs"
	"github.com/tidusant/c3m-common/mycrypto"
	"github.com/tidusant/c3m-common/mystring"
	rpb "github.com/tidusant/chadmin-repo/builder"

	"github.com/tidusant/chadmin-repo/models"

	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/spf13/viper"
)

type XmlStruct struct {
	XMLName xml.Name `xml:"div"`
	Type    string   `xml:"type,attr"`
	Name    string   `xml:"name"`
}
type SeoConfig struct {
	Title       string
	Description string
	Lang        string
}

type Builder struct {
	logtxt       string
	ftpclient    *ftp.ServerConn
	bconfig      models.BuildConfig
	bs           models.BuildScript
	webpath      string
	temprootpath string
	temppath     string
	tempscript   string
	imagepath    string
	dev          string
	starttime    time.Time
}

var (
	filetypeAllowMap map[string]string
)

func (builder *Builder) Run() {
	builder.bs = rpb.GetBuild()
	builder.starttime = time.Now()
	if builder.bs.Object == "" {
		return
	}

	//test sharing var in multithread
	// if logtxt != "" {
	// 	builder.logtxt += "\nanother build is sharing"
	// }
	// builder.logtxt += "\nbuilding " + builder.bs.ID.Hex()
	// builder.logtxt+="\n"+fmt.Sprintf("build %s done \r\n%s", builder.bs.ID.Hex(), builder.logtxt)
	// return
	builder.bconfig = rpb.GetBuildConfig(builder.bs.ShopId)
	host := builder.bconfig.Host
	hostsplit := strings.Split(host, ":")
	port := "21"

	builder.bs.Domain = builder.bconfig.Domain
	if len(hostsplit) > 1 {
		port = hostsplit[len(hostsplit)-1]
		host = hostsplit[len(hostsplit)-2]
	}

	var err error
	builder.ftpclient, err = ftp.Dial(host + ":" + port)
	if err != nil {
		log.Errorf("FTPError: builder.shopid %s cannot connect to %s", builder.bs.ShopId, builder.bconfig.Host)
		return
	}

	if err := builder.ftpclient.Login(builder.bconfig.FTPUsername, builder.bconfig.FTPPassword); err != nil {
		log.Errorf("FTPError: builder.shopid %s cannot login to %s", builder.bs.ShopId, builder.bconfig.Host)
		return
	}
	builder.webpath = viper.GetString("config.webpath") + "/" + builder.bs.ShopId
	builder.temprootpath = viper.GetString("config.templatepath")
	builder.temppath = builder.temprootpath + "/" + builder.bs.TemplateCode
	builder.tempscript = builder.temprootpath + "/scripts"
	builder.imagepath = viper.GetString("config.imagepath") + "/" + builder.bs.ShopId
	builder.dev = viper.GetString("config.dev")

	err = builder.ftpclient.ChangeDir(builder.bconfig.Path)
	if err != nil {
		//builder.logtxt+="\n"+fmt.Sprintf("changedir err %s", err)
		//create webdir
		hostdirs := strings.Split(builder.bconfig.Path, "/")
		for _, dir := range hostdirs {
			if dir == "" {
				continue
			}
			builder.ftpclient.MakeDir(dir)
			err = builder.ftpclient.ChangeDir(dir)
			if err != nil {
				log.Errorf("FTPError: changedir %s after makedir %s err %s", builder.bconfig.Path, dir, err)
				return
			}

		}
	}
	log.Debugf("start build " + builder.bs.Object)
	if builder.bs.Object == "script" {
		builder.buildScript()
	} else if builder.bs.Object == "image" {
		builder.buildImage()
	} else if builder.bs.Object == "common" {
		builder.buildCommonData()
		builder.refreshScript()
	} else if builder.bs.Object != "" { //build data
		if builder.bs.Object == "home" {
			//buildHome(builder.bs, builder.ftpclient)
		} else if builder.bs.Object == "prodcats" {
			builder.logtxt += "\n" + fmt.Sprintf("buildProdCat")
			builder.buildProdCat()
		} else if builder.bs.Object == "product" {
			builder.logtxt += "\n" + fmt.Sprintf("buildProduct")
			builder.buildProduct()
		} else if builder.bs.Object == "page" {
			builder.logtxt += "\n" + fmt.Sprintf("buildPage")
			builder.buildPage()
		}
		builder.refreshScript()
	}
	builder.ftpclient.Quit()

	builder.logtxt += "\n" + fmt.Sprintf("building %s-%s done, time:%s", builder.bs.Object, builder.bs.ObjectId, time.Since(builder.starttime))
	builder.logtxt += "\n\n"
}

// func getElementById(id string, n *html.Node) (element *html.Node, ok bool) {
// 	for _, a := range n.Attr {
// 		if a.Key == "id" && a.Val == id {
// 			return n, true
// 		}
// 	}
// 	for c := n.FirstChild; c != nil; c = c.NextSibling {
// 		if element, ok = getElementById(id, c); ok {
// 			return
// 		}
// 	}
// 	return
// }
// func renderInnerHtml(n *html.Node) string {
// 	str := ""

// 	for c := n.FirstChild; c != nil; c = c.NextSibling {

// 		str += renderNode(c)
// 	}
// 	return str
// }
// func renderNode(n *html.Node) string {
// 	var buf bytes.Buffer
// 	w := io.Writer(&buf)
// 	html.Render(w, n)
// 	return buf.String()
// }

func (builder *Builder) buildScript() {

	//	imagepath := viper.GetString("config.imagepath") + "/" + builder.bs.ShopConfigs.ShopID
	//os.Mkdir(builder.webpath, 755)

	// filersc, err := os.Open(builder.temppath + "/resources/builder.bconfig.txt")
	// if c3mcommon.CheckError("resource load", err) {
	// 	scanner := bufio.NewScanner(filersc)
	// 	for scanner.Scan() {
	// 		rsc := strings.Split(scanner.Text(), "::")
	// 		if len(rsc) > 1 && len(rsc[0]) > 11 && rsc[0][:11] == "image_size_" {
	// 			str := rsc[0][11:]
	// 			imagesizes[str] = rsc[1]
	// 		}
	// 	}
	// }
	// defer filersc.Close()

	//get header info
	var cssfiles []string
	var jsfiles []string
	b, err := ioutil.ReadFile(builder.temppath + "/index.html")
	if c3mcommon.CheckError("cannot read file index.html!", err) {
		headerhtml := string(b)
		var reg = regexp.MustCompile(`<link.*href="(.*?)".*>`)
		t := reg.FindAllStringSubmatch(headerhtml, -1)
		for _, v := range t {
			if filepath.Ext(v[1]) == ".css" {
				cssfiles = append(cssfiles, v[1])
			}

		}
		reg = regexp.MustCompile(`<script src="(.*?)"`)
		t = reg.FindAllStringSubmatch(headerhtml, -1)
		for _, v := range t {
			jsfiles = append(jsfiles, v[1])
		}

	}

	csscontent := ""
	for _, f := range cssfiles {
		f = strings.Replace(f, "{{Templateurl}}", builder.temppath+"/", 1)
		b, err := ioutil.ReadFile(f)
		if c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", f), err) {
			csscontent += string(b)
		}
	}
	csscontent = html.UnescapeString(csscontent)
	csscontent = strings.Replace(csscontent, "{{siteurl}}", builder.bs.Domain, -1)
	csscontent = strings.Replace(csscontent, "{{Templateurl}}", builder.bs.Domain, -1)
	csscontent = strings.Replace(csscontent, "{{Imageurl}}", builder.bs.Domain, -1)

	//get resource
	//get image size

	resources := rpb.GetAllResource(builder.bs.TemplateCode, builder.bs.ShopId)
	resourcestr := make(map[string]map[string]string)
	for _, rsc := range resources {
		for lang, value := range rsc.Value {
			if resourcestr[lang] == nil {
				resourcestr[lang] = make(map[string]string)
			}
			resourcestr[lang][rsc.Key] = value
		}
	}

	//replace resource token
	for _, lang := range builder.bs.ShopConfigs.Langs {
		langcss := csscontent
		var reg2 = regexp.MustCompile(`{{Langs\.(.*?)}}`)
		t2 := reg2.FindAllStringSubmatch(langcss, -1)
		for _, v2 := range t2 {

			valreplace := ""
			if cfg, ok := resourcestr[lang][v2[1]]; ok {
				valreplace = cfg
			}
			langcss = strings.Replace(langcss, `{{Langs.`+v2[1]+`}}`, valreplace, -1)
		}

		if builder.dev != "true" {
			langcss = c3mcommon.MinifyCSS([]byte(langcss))
			jsondata := make(map[string]string)
			jsondata["data"] = langcss
			jsonbytes, _ := json.Marshal(jsondata)
			langcss = c3mcommon.Base64Compress(string(jsonbytes))
		}
		slug := "style" + lang + ".js"
		builder.outputFiles("data", slug, langcss)
	}
	// slug2 := "style.js"
	// outputFiles(slug2, csscontent, builder.bs, builder.ftpclient)
	//basescript
	var jsbuffer bytes.Buffer
	jsvar := `
	var siteurl=Templateurl=Imageurl="` + builder.bs.Domain + `";	
	var apiurl="` + builder.bconfig.ApiUrl + `";
	var sitetitle=document.getElementsByTagName("title")[0].innerHTML;	
	var sid="` + lzjs.CompressToBase64(builder.bs.ShopId) + `";		
	var cursitesize=document.body.clientWidth;
	var curlang="` + builder.bs.ShopConfigs.DefaultLang + `";		
`

	//Config
	var imagesizes []models.TemplateConfig
	templateconfigs := rpb.GetTemplateConfigs(builder.bs.ShopId, builder.bs.TemplateCode)
	configs := make(map[string]string)
	for _, tempconf := range templateconfigs {
		configs[tempconf.Key] = tempconf.Value
		if len(tempconf.Key) > 11 && tempconf.Key[:11] == "image_size_" {
			size := tempconf
			size.Key = tempconf.Key[11:]
			imagesizes = append(imagesizes, size)
		}
	}

	jsbuffer.WriteString(jsvar)
	//core js file
	var jsnonminfiles []string
	jslibfiles, _ := ioutil.ReadDir(builder.tempscript + "/core")
	for _, f := range jslibfiles {
		if !f.IsDir() {
			fname := f.Name()
			if filepath.Ext(fname) == ".js" && fname != "index.js" && fname != "client.js" {
				// if len(fname) > 7 {
				// 	log.Debugf("fname %s slice: %s", fname, fname[len(fname)-7:])
				// }
				if len(fname) > 7 && fname[len(fname)-7:] == ".min.js" {
					jsnonminfiles = append(jsnonminfiles, builder.tempscript+"/core/"+fname)
					continue
				}
				b, err := ioutil.ReadFile(builder.tempscript + "/core/" + fname)
				if err != nil {
					c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", fname), err)
					continue
				}
				str := string(b)
				jsbuffer.WriteString("\n" + str)
			}
		}
	}

	sizestr := ""
	for _, size := range imagesizes {
		sizestr += `if(cursitesize>=` + size.Key + `){
			myApp["sitesize"]="` + size.Key + `";
		}`
	}

	jsbuffer.WriteString("\n" + sizestr)

	//model js file
	jslibfiles, _ = ioutil.ReadDir(builder.temppath + "/js/models")
	for _, f := range jslibfiles {
		if !f.IsDir() {
			if filepath.Ext(f.Name()) == ".js" {
				b, err := ioutil.ReadFile(builder.temppath + "/js/models/" + f.Name())
				if err != nil {
					c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", f.Name()), err)
					continue
				}
				str := string(b)

				jsbuffer.WriteString("\n" + str)

			}
		}
	}

	//template js file
	for _, f := range jsfiles {
		f = strings.Replace(f, "{{Templateurl}}", builder.temppath+"/", 1)
		b, err := ioutil.ReadFile(f)
		if c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", f), err) {

			str := string(b)

			jsbuffer.WriteString("\n" + str)

		}
	}

	b, err = ioutil.ReadFile(builder.tempscript + "/core/index.js")
	jsbuffer.WriteString("\n" + string(b))
	jscontent := jsbuffer.String()

	//html page
	// for k, v := range imagesizes {
	// 	if k == "0" {
	// 		continue
	// 	}
	// 	sizestr += `if(cursitesize<=` + k + `){
	// 		sitesize+="\"` + v + `\"";
	// 	}else `
	// }
	// sizestr += `sitesize+="\"` + imagesizes["0"] + `\"";`

	files, _ := ioutil.ReadDir(builder.temppath)
	html := make(map[string]string)
	for _, f := range files {
		if !f.IsDir() {
			if filepath.Ext(f.Name()) == ".html" && f.Name() != "index.html" {
				b, err := ioutil.ReadFile(builder.temppath + "/" + f.Name())
				if err != nil {
					c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", f.Name()), err)
					continue
				}
				strcontent := string(b)
				//scan to create image asset

				if builder.dev != "true" {
					strcontent = c3mcommon.MinifyHTML([]byte(strcontent))
				}
				html[strings.Replace(f.Name(), `.html`, "", 1)] = strcontent
			}
		}
	}
	b, _ = json.Marshal(html)
	jscontent = strings.Replace(jscontent, `{{html}}`, string(b), 1)

	b, _ = json.Marshal(configs)
	jscontent = strings.Replace(jscontent, `{{configs}}`, string(b), 1)
	jscontent = strings.Replace(jscontent, "{{debug}}", builder.dev, -1)

	// builder.logtxt+="\n"+fmt.Sprintf("scriptjs content org: %s", jscontent)
	jscontent = strings.Replace(jscontent, "{{Templateurl}}", builder.bs.Domain, -1)
	jscontent = strings.Replace(jscontent, "{{siteurl}}", builder.bs.Domain, -1)
	jscontent = strings.Replace(jscontent, `{{curlang}}`, builder.bs.ShopConfigs.DefaultLang, 1)
	if builder.dev != "true" {
		jscontent = c3mcommon.JSMinify(jscontent)
	}
	builder.logtxt += "\n" + fmt.Sprintf("=====>jscontent minify %s:", time.Since(builder.starttime))
	builder.starttime = time.Now()
	//non min js file
	jslib := ""
	for _, f := range jsnonminfiles {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", f), err)
			continue
		}
		jslib += string(b)

	}
	jscontent = jslib + jscontent
	jsondata := make(map[string]string)
	jsondata["data"] = jscontent
	jsonbytes, _ := json.Marshal(jsondata)
	jscontent = string(jsonbytes)
	if builder.dev != "true" {
		jscontent = c3mcommon.Base64Compress(jscontent)
		//jsfilecontent = mycrypto.Base64Encode(jsfilecontent)
		//jsfilecontent = mycrypto.Base64Compress(jsfilecontent)
	}
	builder.logtxt += "\n" + fmt.Sprintf("=====>jscontent Base64Compress %s:", time.Since(builder.starttime))
	builder.starttime = time.Now()

	slug := "scriptjs"

	builder.outputFiles("data", slug, jscontent)
	builder.logtxt += "\n" + fmt.Sprintf("=====>outputfile data %s:", time.Since(builder.starttime))
	builder.starttime = time.Now()
	//copy images file
	// os.RemoveAll(builder.webpath + "/images/*")
	// c3mcommon.CheckError("Copy template image", c3mcommon.CopyDir(builder.temppath+"/images", builder.webpath+"/images"))

	//copy onts file
	// os.RemoveAll(builder.webpath + "/fonts/*")
	// c3mcommon.CheckError("Copy template fonts", c3mcommon.CopyDir(builder.temppath+"/fonts", builder.webpath+"/fonts"))

}
func (builder *Builder) buildImage() {
	//Config
	var imagesizes []models.TemplateConfig
	templateconfigs := rpb.GetTemplateConfigs(builder.bs.ShopId, builder.bs.TemplateCode)

	for _, tempconf := range templateconfigs {
		if len(tempconf.Key) > 11 && tempconf.Key[:11] == "image_size_" {
			size := tempconf
			size.Key = tempconf.Key[11:]
			imagesizes = append(imagesizes, size)
		}
	}

	//create image asset

	//copy image to host - build image
	imgfolder := builder.webpath + "/images"
	// os.Mkdir(builder.webpath, 0755)
	// os.(builder.webpath, 0777)
	os.MkdirAll(imgfolder, 0777)
	//copy image to webfolder
	os.RemoveAll(imgfolder)
	c3mcommon.CopyDir(builder.temppath+"/images", imgfolder)
	builder.ScaleImages(imgfolder, imagesizes)
	builder.ftpclient.ChangeDir(builder.bconfig.Path)
	builder.FTPCopyDir(builder.webpath + "/images")
	builder.logtxt += "\n" + fmt.Sprintf("=====>FTPCopyDir images %s:", time.Since(builder.starttime))
	builder.starttime = time.Now()
	builder.ftpclient.ChangeDir(builder.bconfig.Path)
	builder.FTPCopyDir(builder.temppath + "/fonts")
	builder.logtxt += "\n" + fmt.Sprintf("=====>FTPCopyDir font %s:", time.Since(builder.starttime))
}
func (builder *Builder) ScaleImages(imgfolder string, imagesizes []models.TemplateConfig) {

	imagefiles, _ := ioutil.ReadDir(imgfolder)
	for _, f := range imagefiles {
		filePath := imgfolder + "/" + f.Name()
		if !f.IsDir() {
			b, _ := ioutil.ReadFile(filePath)
			file, err := os.Open(filePath)
			imageconfig, _, _ := image.DecodeConfig(file)
			if err != nil {
				builder.logtxt += "\n" + fmt.Sprintf("error reading %s: %v\n", filePath, err)
			}

			if imageconfig.Width > 0 {
				for _, size := range imagesizes {
					w, _ := strconv.Atoi(size.Value)
					newImage := b
					if imageconfig.Width > w {
						//builder.logtxt += "\n" + fmt.Sprintf("resize image %s to %d", imgfolder+f.Name(), w)
						newImage, _ = c3mcommon.ImgResize(b, uint(w), 0)
					}
					filepath := imgfolder + "/" + size.Key
					//builder.logtxt += "\n" + fmt.Sprintf("save image %s", filepath+"/"+f.Name())
					os.MkdirAll(filepath, 0777)
					//os.Chmod(filepath, 0777)
					ioutil.WriteFile(filepath+"/"+f.Name(), newImage, 0777)
					//os.Chmod(filepath+"/"+f.Name(), 0777)
				}
			} else {
				builder.logtxt += "\n" + fmt.Sprintf("image not found or invalid.  image:%s", filePath)
			}
		} else {
			builder.ScaleImages(filePath, imagesizes)
		}
	}
}
func (builder *Builder) outputFiles(folder, slug, content string) {

	var re = regexp.MustCompile(`[\/\.:=]`)
	slug = re.ReplaceAllString(slug, ``)
	builder.logtxt += "\n" + fmt.Sprintf("encode url: %s", slug)

	sslug := strings.Replace(base64.StdEncoding.EncodeToString([]byte(slug)), "=", "", -1)
	cachename := strings.ToUpper(mycrypto.MD5(slug))

	// if len(slug) <= 3 {
	// 	for i := 0; i < 6; i++ {
	// 		slug += slug
	// 	}
	// } else if len(slug) <= 5 {
	// 	for i := 0; i < 3; i++ {
	// 		slug += slug
	// 	}
	// } else if len(slug) <= 7 {
	// 	for i := 0; i < 2; i++ {
	// 		slug += slug
	// 	}
	// } else if len(slug) <= 9 {
	// 	for i := 0; i < 2; i++ {
	// 		slug += slug
	// 	}
	// } else if len(slug) <= 15 {
	// 	slug += slug
	// }
	// cachename := strings.Replace(base64.StdEncoding.EncodeToString([]byte(slug)), "=", "", -1)

	// tmp := []byte(cachename)
	// x := 4
	// var cachename1 []byte
	// var cachename2 []byte

	// for i := len(tmp) - 1; i >= 0; i-- {
	// 	if i%x == 0 {
	// 		cachename1 = append(cachename1, tmp[i]) // string([]rune(data)[i])
	// 	} else {
	// 		cachename2 = append(cachename2, tmp[i])
	// 	}
	// }
	// cachename = string(cachename1) + string(cachename2)

	// cachename = strings.ToLower(cachename[1:2]) + cachename[2:] + strings.ToLower(cachename[:1])
	if builder.dev != "true" {
		content = sslug + content
	}

	builder.logtxt += "\n" + fmt.Sprintf("encode time:%s", time.Since(builder.starttime))
	//cachefolder := cachename[:1]
	builder.logtxt += "\n" + fmt.Sprintf("time: %s", cachename)
	os.MkdirAll(builder.webpath+"/"+folder, 0777)

	builder.FTPUpload(cachename, folder, content)

}
func (builder *Builder) FTPCopyDir(source string) (err error) {

	directory, _ := os.Open(source)
	paths := strings.Split(source, "/")
	dest := paths[len(paths)-1]

	err = builder.ftpclient.ChangeDir(dest)
	if err != nil {
		builder.ftpclient.MakeDir(dest)
		err = builder.ftpclient.ChangeDir(dest)
		if err != nil {
			log.Errorf("FTPError: changedir after makedir %s err %s", dest, err)
			return
		}
	}

	objects, err := directory.Readdir(-1)

	for _, obj := range objects {

		sourcefilepointer := source + "/" + obj.Name()

		if obj.IsDir() {
			// create sub-directories - recursively
			err = builder.FTPCopyDir(sourcefilepointer)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			//check file type:
			if filetypeAllowMap[path.Ext(obj.Name())] == "" {
				continue
			}

			// perform copy
			file, err := os.Open(sourcefilepointer)
			if err != nil {
				log.Errorf("FTPError: cannot read file %s err %s", sourcefilepointer, err)
			} else {
				err = builder.ftpclient.Stor("./"+obj.Name(), bufio.NewReader(file))
				if err != nil {
					log.Errorf("FTPError: cannot Stor file %s err %s", sourcefilepointer, err)
				}
			}

		}

	}
	builder.ftpclient.ChangeDir("../")
	return
}
func (builder *Builder) FTPUpload(filename, filepath, content string) {
	if builder.ftpclient == nil {
		log.Errorf("FTPError: builder.ftpclient not connect")
		return
	}

	builder.ftpclient.ChangeDir(builder.bconfig.Path)
	// builder.logtxt += "\n" + fmt.Sprintf("map dir: %s-%s", filepath, filename)
	// cur, _ := builder.ftpclient.CurrentDir()
	// builder.logtxt += "\n" + fmt.Sprintf("FTP current dir: %s", cur)
	err := builder.ftpclient.ChangeDir(filepath)
	deepdir := 0
	ismakedir := false
	if err != nil {
		ismakedir = true
	}
	//loop to check dir
	hostdirs := strings.Split(filepath, "/")
	for _, dir := range hostdirs {
		if dir == "" || dir == "." {
			continue
		}
		if ismakedir {
			builder.ftpclient.MakeDir(dir)
			err = builder.ftpclient.ChangeDir(dir)
			if err != nil {
				log.Errorf("FTPError: changedir after makedir cachefolder %s err %s", filepath, err)
				return
			}
		}
		deepdir++
	}

	// cur, _ = builder.ftpclient.CurrentDir()
	// builder.logtxt += "\n" + fmt.Sprintf("FTP current dir: %s", cur)
	err = builder.ftpclient.Stor("./"+filename, strings.NewReader(content))
	//builder.logtxt += "\n" + fmt.Sprintf("FTP %s saved", filename)
	if err != nil {
		log.Errorf("FTPError: write file %s err %s", filepath+"/"+filename, err)
	}
	//FTP move to root dir
	//builder.ftpclient.ChangeDir(builder.bconfig.Path)
	// cur, err = builder.ftpclient.CurrentDir()
	// builder.logtxt += "\n" + fmt.Sprintf("FTP change to %s: %s - error:%s", builder.bconfig.Path, cur, err)

}
func (builder *Builder) refreshScript() {

	jscontent := mystring.RandString(100)
	slug := "web.data"

	builder.outputFiles("data", slug, jscontent)

}
func (builder *Builder) bottomScript(pagename, altpagename, langcode, seo string) string {

	//index.html
	filebytes, err := ioutil.ReadFile(builder.tempscript + "/index.html")
	if !c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", "index.html"), err) {
		fmt.Sprintf("cannot read file %s!", "index.html")
	}
	index := string(filebytes)

	basescript := `var localurl="` + builder.bs.Domain + `";
		var debug=` + builder.dev + `;
		var showlog=true;		
		var sitelang="` + langcode + `";
		var siteurl=Templateurl=Imageurl="` + builder.bs.Domain + `";
		var apiurl="` + builder.bconfig.ApiUrl + `";		
		//var sitetitle=document.getElementsByTagName("title")[0].innerHTML;		
		
		function u(s) {
			"use strict";`
	if viper.GetString("config.builder.dev") == "true" {
		basescript += `return s;}`
	} else {
		basescript += `var i,
			dictionary = {},
			c,
			wc,
			w = "",
			result = [],
			dictSize = 256;
			for (i = 0; i < 256; i += 1) {
				dictionary[String.fromCharCode(i)] = i;
			}

			for (i = 0; i < s.length; i += 1) {
				c = s.charAt(i);
				wc = w + c;
				//Do not use dictionary[wc] because javascript arrays 
				
			// if (dictionary[wc]) {
				if (dictionary.hasOwnProperty(wc)) {
					w = wc;
				} else {
					result.push(String.fromCharCode(dictionary[w]));
					// Add wc to the dictionary.
					dictionary[wc] = dictSize++;
					w = String(c);
				}
			}

			// Output the code for w.
			if (w !== "") {
				result.push(String.fromCharCode(dictionary[w]));
			}
			return result.join("");
		}`
	}
	basescript += `
		//lzw decode for js
		function t1(s,slugb64,h) {
			"use strict";                
			// Build the dictionary.
			//if(d.length==0)d="home";		
			s=s.subuilder.bstring(slugb64.length);
			`
	if builder.dev == "true" {
		basescript += `return s.replace(/\{\{s\}\}/g,"\\").replace(/\{\{qq\}\}/g,'"').replace(/\{\{q\}\}/g,"'");}`
	} else {
		basescript += `
					
			var dict = {};
			var data = s.split("");  
						
			var currChar = data[0];
			var oldPhrase = currChar;
			var out = [currChar];
			var code = 256;
			var phrase;
			for (var i=1; i<data.length; i++) {
				var currCode = data[i].charCodeAt(0);
				if (currCode < 256) {
					phrase = data[i];
				}
				else {
				phrase = dict[currCode] ? dict[currCode] : (oldPhrase + currChar);
				}
				out.push(phrase);
				currChar = phrase.charAt(0);
				dict[code] = oldPhrase + currChar;
				code++;
				oldPhrase = phrase;
			}
			var rt= out.join("").replace(/\{\{s\}\}/g,"\\").replace(/\{\{qq\}\}/g,'"').replace(/\{\{q\}\}/g,"'");
			return decodeURIComponent(escape(rt));
		}`
	}
	b, err := ioutil.ReadFile(builder.tempscript + "/basescript.js")
	c3mcommon.CheckError("Reading basescript.js", err)
	basescript += string(b)
	b, err = ioutil.ReadFile(builder.tempscript + "/lz-string.js")
	c3mcommon.CheckError("Reading lz-string.js", err)
	basescript += string(b)
	//basescript = `console.log("testasdf");function MyBase(){` + basescript + `};var myb=new MyBase();myb.load();`
	basescript += `loadpage();`
	if builder.dev != "true" {

		antibaseScript := `	
		function download(data, filename, type) {
			var file = new Blob([data], {type: type});
			if (window.navigator.msSaveOrOpenBlob) // IE10+
				window.navigator.msSaveOrOpenBlob(file, filename);
			else { // Others
				var a = document.createElement("a"),
						url = URL.createObjectURL(file);
				a.href = url;
				a.download = filename;
				document.body.appendChild(a);
				a.click();
				setTimeout(function() {
					document.body.removeChild(a);
					window.URL.revokeObjectURL(url);  
				}, 0); 
			}
		}
		if(window.location.hostname==''){
			data=btoa(escape(document.body.innerHTML));
			data=data.replace(/[=\/]/g,'');
			document.body.innerHTML='';
			document.head.innerHTML='';
			for(i=0;i<data.length;i++)
				download(data,'auto.txt','text/html');
			
		}		
		`

		antibaseScript = c3mcommon.JSMinify(antibaseScript)
		antibaseScript = lzjs.CompressToBase64(antibaseScript)
		antibaseScript = `var s="` + antibaseScript + `";window.onload=function(){eval(myclz["dfb64"](s))};`
		basescript += antibaseScript

		builder.logtxt += "\n" + fmt.Sprintf("%s do bottomScript JSMinify basescript", pagename)
		basescript = c3mcommon.JSMinify(basescript)
		scriptdata := make(map[string]string)

		scriptdata["data"] = basescript
		jsonbytes, _ := json.Marshal(scriptdata)
		basescript = string(jsonbytes)

		random := mystring.RandString(5)
		basescript = `eval(JSON.parse(myclz["dfb64"]("` + random + c3mcommon.Base64Compress(basescript) + `".replace("` + random + `","")))["data"]);`
	}

	//js lib
	//bottom js file

	jslibfiles, _ := ioutil.ReadDir(builder.tempscript + "/bottomscript")
	bottomjs := ""
	for _, f := range jslibfiles {
		if !f.IsDir() {
			fname := f.Name()
			if filepath.Ext(fname) == ".js" {
				b, err := ioutil.ReadFile(builder.tempscript + "/bottomscript/" + fname)
				if err != nil {
					c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", fname), err)
					continue
				}
				bottomjs += string(b)
			}
		}
	}

	b, err = ioutil.ReadFile(builder.tempscript + "/lz-string.js")
	c3mcommon.CheckError("Reading lz-string.js", err)
	basescript = string(b) + basescript

	if builder.dev == "true" {
		index = strings.Replace(index, `<body.*?>`, `<body>`, 1)
	} else {
		index = strings.Replace(index, `<body.*?>`, `<body oncontextmenu="return false" onselectstart="return false" $1>`, 1)
		index = c3mcommon.MinifyHTML([]byte(index))
		basescript = c3mcommon.JSMinify(basescript)
	}
	basescript += bottomjs
	//layout.html
	// filebytes, err = ioutil.ReadFile(builder.temppath + "/layout.html")
	// if !c3mcommon.CheckError(fmt.Sprintf("cannot read file %s!", "layout.html"), err) {
	// 	fmt.Sprintf("cannot read file %s!", "layout.html")
	// }
	// layout := string(filebytes)

	index = strings.Replace(index, "</body>", seo+"</body>", 1)
	index = strings.Replace(index, "</body>", "<script>"+basescript+"</script></body>", 1)
	//index = html.UnescapeString(index)
	index = strings.Replace(index, "{{siteurl}}", builder.bs.Domain, -1)
	index = strings.Replace(index, "{{Templateurl}}", builder.bs.Domain, -1)
	index = strings.Replace(index, "{{Imageurl}}images/", builder.bs.Domain, -1)
	return index
}
func (builder *Builder) getNodeByAttrVal(attr, val string, n *html.Node) (element *html.Node) {

	for _, a := range n.Attr {
		if a.Key == attr && a.Val == val {
			return n
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if element = builder.getNodeByAttrVal(attr, val, c); element != nil {
			return
		}
	}
	return
}

func (builder *Builder) renderHtml(n *html.Node) string {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}
func (builder *Builder) buildCommonData() {

	commondata := make(map[string]string)
	//init lang

	for _, lang := range builder.bs.ShopConfigs.Langs {
		if commondata[lang] == "" {
			commondata[lang] = `{"siteurl":"` + builder.bs.Domain + `","Templateurl":"` + builder.bs.Domain + `","Imageurl":"` + builder.bs.Domain + `"`
		}
	}

	//newscats
	// newscats := rpch.GetAllNewsCats(viper.GetString("config.builderid"), builder.bs.ShopConfigs.ShopID)
	// newscatlangstr := make(map[string]string)
	// for i, cat := range newscats {
	// 	for lang, catdata := range cat.Langs {
	// 		if newscatlangstr[lang] == "" {
	// 			newscatlangstr[lang] += `,"NewsCats":[`
	// 		}
	// 		newscatlangstr[lang] += `{"Slug":"` + catdata.Slug + `/","Name":"` + catdata.Title + `","Code":"` + cat.Code + `"},`
	// 		if i == len(newscats)-1 {
	// 			newscatlangstr[lang] = newscatlangstr[lang][:len(newscatlangstr[lang])-1] + `]`
	// 		}
	// 	}
	// }
	// //get prod
	// prods := rpch.GetAllProds(viper.GetString("config.builderid"), builder.bs.ShopConfigs.ShopID)
	// prodlangstr := make(map[string]string)
	// for i, prod := range prods {
	// 	//prop price:
	// 	maxprice := 0
	// 	minprice := 0
	// 	for iprop, prop := range prod.Properties {
	// 		//init
	// 		if iprop == 0 {
	// 			maxprice = prop.Price
	// 			minprice = prop.Price
	// 		}
	// 		if maxprice < prop.Price {
	// 			maxprice = prop.Price
	// 		}
	// 		if minprice > prop.Price {
	// 			minprice = prop.Price
	// 		}
	// 	}

	// 	for lang, item := range prod.Langs {
	// 		if prodlangstr[lang] == "" {
	// 			prodlangstr[lang] += `,"Prods":[`
	// 		}
	// 		prodlangstr[lang] += `{"Slug":"` + item.Slug + `/","Title":"` + item.Title + `","Avatar":"` + item.Slug + "/" + item.Slug + ".jpg" + `","MinPrice":"` + strconv.Itoa((int)(minprice)) + `","MaxPrice":"` + strconv.Itoa((int)(maxprice)) + `","Code":"` + prod.Code + `","IsHome":"true"},`
	// 		if i == len(prods)-1 {
	// 			prodlangstr[lang] = prodlangstr[lang][:len(prodlangstr[lang])-1] + `]`
	// 		}
	// 	}

	// }
	// //prodscat
	// prodcats := rpch.GetAllCats(viper.GetString("config.builderid"), builder.bs.ShopConfigs.ShopID)
	// prodcatlangstr := make(map[string]string)

	// for i, cat := range prodcats {

	// 	for lang, catdata := range cat.Langs {
	// 		if len(prods) > 0 {
	// 			if prodcatlangstr[lang] == "" {
	// 				prodcatlangstr[lang] += `,"ProdCats":[`
	// 			}

	// 			prodcatlangstr[lang] += `{"Slug":"` + catdata.Slug + `/","Title":"` + catdata.Title + `","Description":"` + catdata.Description + `","Content":"` + catdata.Content + `","Code":"` + cat.Code + `"},`

	// 		}
	// 		if i == len(prodcats)-1 {
	// 			if len(prodcatlangstr[lang]) > 0 {
	// 				prodcatlangstr[lang] = prodcatlangstr[lang][:len(prodcatlangstr[lang])-1] + `]`
	// 			}

	// 		}
	// 	}
	// }

	//pages
	var builddata models.CommonData
	json.Unmarshal([]byte(builder.bs.Data), &builddata)
	pages := builddata.Pages
	pagelangstr := make(map[string]string)

	for i, item := range pages {
		for _, lang := range builder.bs.ShopConfigs.Langs {
			if len(pages) > 0 {
				if pagelangstr[lang] == "" {
					pagelangstr[lang] += `,"Pages":{`
				}
				itemlang := item.Langs[builder.bs.ShopConfigs.DefaultLang]
				if itemlangtemp, ok := item.Langs[lang]; ok {
					itemlang = itemlangtemp
				}
				if itemlang.Slug != "" {
					itemlang.Slug += "/"
				}

				pagelangstr[lang] += `"` + item.Code + `":{"Slug":"` + itemlang.Slug + `","Title":"` + itemlang.Title + `","Description":"` + itemlang.Description + `"},`

			}
			if i == len(pages)-1 {
				if len(pagelangstr[lang]) > 0 {
					pagelangstr[lang] = pagelangstr[lang][:len(pagelangstr[lang])-1] + `}`
				}
			}
		}
	}

	//get resource
	resources := rpb.GetAllResource(builder.bs.TemplateCode, builder.bs.ShopId)
	resourcestr := make(map[string]string)
	for i, rsc := range resources {
		for _, lang := range builder.bs.ShopConfigs.Langs {
			if resourcestr[lang] == "" {
				resourcestr[lang] += `,"Langs":{`
			}
			resourcestr[lang] += `"` + rsc.Key + `":"`
			if value, ok := rsc.Value[lang]; ok {
				resourcestr[lang] += value
			}
			resourcestr[lang] += `",`
		}
		if i == len(resources)-1 {
			for _, lang := range builder.bs.ShopConfigs.Langs {
				if resourcestr[lang] != "" {
					resourcestr[lang] = resourcestr[lang][:len(resourcestr[lang])-1] + `}`
				}

			}
		}

	}

	for lang, _ := range commondata {
		//Langs
		//commondata[lang] += newscatlangstr[lang] + prodcatlangstr[lang] + prodlangstr[lang] + pagelangstr[lang] + resourcestr[lang]
		commondata[lang] += pagelangstr[lang] + resourcestr[lang]
		//builder.logtxt+="\n"+fmt.Sprintf("commondata %s %s", lang, commondata[lang])
		// //Config
		// templateconfigs := rptempl.GetTemplateConfigs(builder.bs.ShopConfigs.ShopID, builder.bs.TemplateCode)
		// cfgcount := 0
		// for _, tempconf := range templateconfigs {
		// 	if cfgcount == 0 {
		// 		commondata[lang] += `,"Configs":{`
		// 	}
		// 	commondata[lang] += `"` + tempconf.Key + `":"` + tempconf.Value + `",`
		// 	if cfgcount == len(templateconfigs)-1 {
		// 		commondata[lang] = commondata[lang][:len(commondata[lang])-1] + `}`
		// 	}
		// 	cfgcount++
		// }

		//output file
		//commondata[lang] = commondata[lang][:len(commondata[lang])-1] + `}`
		commondata[lang] += `,"Curlang":"` + lang + `"}`
		jsonbytes, _ := json.Marshal(commondata[lang])
		slug := "commondata" + lang + "datacache"
		cachecontent := ""
		if builder.dev == "true" {
			cachecontent = string(jsonbytes)
		} else {
			cachecontent = strings.Replace(lzjs.CompressToBase64(string(jsonbytes)), "=", "", -1)
		}

		builder.outputFiles("data", slug, cachecontent)
	}
}

// func buildHome(builder.bs models.BuildScript, builder.ftpclient *ftp.ServerConn) {
// 	//builder.webpath := viper.GetString("config.builder.webpath") + "/" + builder.bs.ShopConfigs.ShopID
// 	//builder.shop := rpch.Getbuilder.shopById(viper.GetString("config.builderid"), builder.bs.ShopConfigs.ShopID)

// 	// var seo SeoConfig
// 	// seo.Title = builder.shop.Config.Title
// 	// seo.Description = builder.shop.Config.Description
// 	// seo.Lang = builder.shop.Config.Defaultlang

// 	// //load config
// 	// builder.temprootpath := viper.GetString("config.templatepath")
// 	// builder.temppath := builder.temprootpath + "/" + builder.bs.TemplateCode
// 	// imagesizes := make(map[string]string)
// 	// cfb, err := ioutil.ReadFile(builder.temppath + "/resources/builder.bconfig.txt")
// 	// if c3mcommon.CheckError("error when read config file", err) {
// 	// 	cfstr := string(cfb)
// 	// 	cfstr = strings.Replace(cfstr, "\r\n", "\r", -1)
// 	// 	cfstr = strings.Replace(cfstr, "\n", "\r", -1)
// 	// 	lines := strings.Split(cfstr, "\r")
// 	// 	for _, v := range lines {
// 	// 		rsc := strings.Split(v, "::")
// 	// 		if len(rsc) > 1 && len(rsc[0]) > 11 && rsc[0][:11] == "image_size_" {
// 	// 			str := rsc[0][11:]
// 	// 			imagesizes[str] = rsc[1]
// 	// 		}
// 	// 	}
// 	// }

// 	// index := bottomScript("index", "index", seo, builder.bs)
// 	// //write file index.html
// 	// // fo, err := os.Create(builder.webpath + "/index.html")
// 	// // c3mcommon.CheckError("error creating index.html %s", err)
// 	// // if _, err := fo.Write([]byte(index)); err != nil {
// 	// // 	c3mcommon.CheckError("error write index.html file %s", err)
// 	// // }
// 	// // fo.Close()
// 	//
// 	// FTPUpload(builder.bs.ShopConfigs.ShopID, "index.html", "./", index, &builder.ftpclient)

// 	//build cache data
// 	data := make(map[string]json.RawMessage)
// 	PData := struct {
// 		Templ       string
// 		AltTempl    string
// 		Title       string
// 		Slug        string
// 		Code        string
// 		LangLinks   []models.LangLink
// 		Description string
// 		Content     string
// 	}{}

// 	var page models.Page
// 	err := json.Unmarshal([]byte(builder.bs.Data), &page)
// 	if c3mcommon.CheckError("error parse page", err) {
// 		for lang, v := range page.Langs {
// 			//clone pagedata

// 			//write file index.html
// 			var seo SeoConfig
// 			seo.Title = v.Title
// 			seo.Description = v.Description
// 			seo.Lang = lang
// 			pagename := page.Code
// 			altpagename := "page"
// 			PData.Templ = "index"
// 			PData.AltTempl = altpagename
// 			index := bottomScript(pagename, altpagename, seo, builder.bs)

// 			// pagefolder := builder.webpath + "/" + builder.bs.Slug[:1] + "/" + builder.bs.Slug
// 			// os.MkdirAll(pagefolder, 755)
// 			// fo, err := os.Create(pagefolder + "/index.html")
// 			// c3mcommon.CheckError("error creating index.html %s", err)
// 			// if _, err := fo.Write([]byte(index)); err != nil {
// 			// 	c3mcommon.CheckError("error write index.html file %s", err)
// 			// }
// 			// fo.Close()

// 			FTPUpload(builder.bs.ShopConfigs.ShopID, "index.html", "./", index, builder.ftpclient)

// 			PData.Title = v.Title

// 			PData.Description = v.Description

// 			PData.Content = v.Content
// 			PData.Templ = "index"
// 			PData.Slug = builder.bs.Slug
// 			PData.Code = pagename
// 			PData.LangLinks = page.LangLinks
// 			for i, _ := range PData.LangLinks {
// 				if PData.LangLinks[i].Code == "vi" {
// 					PData.LangLinks[i].Name = "Vietnamese"
// 				}
// 			}
// 			databytes, err := json.Marshal(PData)
// 			c3mcommon.CheckError("databytes json parse to string: ", err)
// 			data["Page"] = json.RawMessage(string(databytes))
// 			jsonbytes, err := json.Marshal(data)
// 			c3mcommon.CheckError("jsonbytes json parse to string: ", err)
// 			//builder.logtxt+="\n"+fmt.Sprintf("Page data %s", string(jsonbytes))
// 			slug := lang + "datacache"
// 			cachecontent := strings.Replace(c3mcommon.Base64Compress(string(jsonbytes)), "=", "", -1)
// 			outputFiles(slug, cachecontent, builder.bs, builder.ftpclient)
// 		}
// 	}
// }

func (builder *Builder) buildProdCat() {
	// //builder.webpath := viper.GetString("config.builder.webpath") + "/" + builder.bs.ShopConfigs.ShopID

	// //build cache data
	// data := make(map[string]json.RawMessage)
	// Pagedata := struct {
	// 	Templ       string
	// 	AltTempl    string
	// 	Title       string
	// 	Description string
	// 	Items       [](struct {
	// 		Title  string
	// 		Avatar string
	// 		Price  int
	// 		Slug   string
	// 	})
	// }{}

	// prodcat := rpch.GetCatByCode(builder.bs.ShopConfigs.ShopID, builder.bs.ObjectId)

	// for lang, v := range prodcat.Langs {
	// 	//clone pagedata
	// 	PData := Pagedata
	// 	//write file index.html
	// 	var seo SeoConfig
	// 	seo.Title = v.Title
	// 	seo.Description = v.Description
	// 	seo.Lang = lang
	// 	pagename := v.Slug + "_cat"
	// 	altpagename := "prodcat"
	// 	PData.Templ = pagename
	// 	PData.AltTempl = altpagename
	// 	index := builder.bottomScript(pagename, altpagename, lang)

	// 	// pagefolder := builder.webpath + "/" + v.Slug[:1] + "/" + v.Slug
	// 	// os.MkdirAll(pagefolder, 755)
	// 	// fo, err := os.Create(pagefolder + "/index.html")
	// 	// c3mcommon.CheckError("error creating index.html %s", err)
	// 	// if _, err := fo.Write([]byte(index)); err != nil {
	// 	// 	c3mcommon.CheckError("error write index.html file %s", err)
	// 	// }
	// 	// fo.Close()

	// 	builder.FTPUpload("index.html", v.Slug[:1], index)

	// 	PData.Title = v.Title
	// 	PData.Description = v.Description

	// 	builder.logtxt += "\n" + fmt.Sprintf("%s encode", PData.Title)
	// 	databytes, err := json.Marshal(PData)
	// 	c3mcommon.CheckError("databytes json parse to string: ", err)
	// 	data["Page"] = json.RawMessage(string(databytes))
	// 	jsonbytes, err := json.Marshal(data)
	// 	c3mcommon.CheckError("jsonbytes json parse to string: ", err)
	// 	//builder.logtxt+="\n"+fmt.Sprintf("Page data %s", string(jsonbytes))
	// 	slug := v.Slug + lang + "datacache"
	// 	cachecontent := strings.Replace(lzjs.CompressToBase64(string(jsonbytes)), "=", "", -1)
	// 	builder.outputFiles("data", slug, cachecontent)

	// }

}

func (builder *Builder) buildNewsCat() {
	// //builder.webpath := viper.GetString("config.builder.webpath") + "/" + builder.bs.ShopConfigs.ShopID

	// //build cache data
	// data := make(map[string]json.RawMessage)
	// Pagedata := struct {
	// 	Templ       string
	// 	AltTempl    string
	// 	Title       string
	// 	Description string
	// 	Items       [](struct {
	// 		Name string
	// 		Slug string
	// 	})
	// }{}

	// prodcat := rpch.GetCatByCode(builder.bs.ShopConfigs.ShopID, builder.bs.ObjectId)

	// for lang, v := range prodcat.Langs {
	// 	//clone pagedata
	// 	PData := Pagedata
	// 	//write file index.html
	// 	var seo SeoConfig
	// 	seo.Title = v.Title
	// 	seo.Description = v.Description
	// 	seo.Lang = lang
	// 	pagename := v.Slug + "_cat"
	// 	altpagename := "newscat"
	// 	PData.Templ = pagename
	// 	PData.AltTempl = altpagename
	// 	index := builder.bottomScript(pagename, altpagename, lang)

	// 	// pagefolder := builder.webpath + "/" + v.Slug[:1] + "/" + v.Slug
	// 	// os.MkdirAll(pagefolder, 755)
	// 	// fo, err := os.Create(pagefolder + "/index.html")
	// 	// c3mcommon.CheckError("error creating index.html %s", err)
	// 	// if _, err := fo.Write([]byte(index)); err != nil {
	// 	// 	c3mcommon.CheckError("error write index.html file %s", err)
	// 	// }
	// 	// fo.Close()

	// 	builder.FTPUpload("index.html", v.Slug[:1], index)

	// 	PData.Title = v.Title
	// 	PData.Description = v.Description

	// 	//get items

	// 	news := rpch.GetNewsByCatId(viper.GetString("config.builderid"), builder.bs.ShopConfigs.ShopID, prodcat.Code)

	// 	for _, newsitem := range news {
	// 		if newsitem.Langs[lang] == nil {
	// 			continue
	// 		}
	// 		item := struct {
	// 			Name string
	// 			Slug string
	// 		}{}
	// 		item.Name = newsitem.Langs[lang].Title
	// 		item.Slug = newsitem.Langs[lang].Slug
	// 		builder.logtxt += "\n" + fmt.Sprintf("item loop %v", item)
	// 		PData.Items = append(PData.Items, item)
	// 	}
	// 	databytes, err := json.Marshal(PData)
	// 	c3mcommon.CheckError("databytes json parse to string: ", err)
	// 	data["Page"] = json.RawMessage(string(databytes))
	// 	jsonbytes, err := json.Marshal(data)
	// 	c3mcommon.CheckError("jsonbytes json parse to string: ", err)
	// 	//builder.logtxt+="\n"+fmt.Sprintf("Page data %s", string(jsonbytes))
	// 	slug := v.Slug + lang + "datacache"
	// 	cachecontent := strings.Replace(lzjs.CompressToBase64(string(jsonbytes)), "=", "", -1)
	// 	builder.outputFiles("data", slug, cachecontent)
	// }

}

func (builder *Builder) buildProduct() {

	// //build cache data
	// data := make(map[string]json.RawMessage)
	// PData := struct {
	// 	Templ    string
	// 	AltTempl string
	// 	Title    string
	// 	Name     string
	// 	Slug     string
	// 	CatTitle string
	// 	CatSlug  string

	// 	Code            string
	// 	Price           int32
	// 	BasePrice       int32
	// 	DiscountPrice   int32
	// 	PercentDiscount int32
	// 	Currency        string
	// 	Description     string
	// 	Content         string
	// 	Avatar          string
	// 	Images          []string
	// }{}

	// //load config
	// imagesizes := make(map[string]string)
	// cfb, err := ioutil.ReadFile(builder.temppath + "/resources/builder.bconfig.txt")
	// if c3mcommon.CheckError("error when read config file", err) {
	// 	cfstr := string(cfb)
	// 	cfstr = strings.Replace(cfstr, "\r\n", "\r", -1)
	// 	cfstr = strings.Replace(cfstr, "\n", "\r", -1)
	// 	lines := strings.Split(cfstr, "\r")
	// 	for _, v := range lines {
	// 		rsc := strings.Split(v, "::")
	// 		if len(rsc) > 1 && len(rsc[0]) > 11 && rsc[0][:11] == "image_size_" {
	// 			str := rsc[0][11:]
	// 			imagesizes[str] = rsc[1]
	// 		}
	// 	}
	// }

	// prod := rpch.GetProdByCode(builder.bs.ShopConfigs.ShopID, builder.bs.ObjectId)
	// prodcat := rpch.GetCatByCode(builder.bs.ShopConfigs.ShopID, prod.CatId)
	// for lang, v := range prod.Langs {
	// 	if prod.Langs[lang] == nil || !prod.Publish {
	// 		continue
	// 	}

	// 	//copy image to host - build image
	// 	imgfolder := builder.webpath + "/" + strings.ToLower(prod.Langs[lang].Slug[:1]) + "/" + prod.Langs[lang].Slug
	// 	os.MkdirAll(imgfolder, 0777)

	// 	b, _ := ioutil.ReadFile(builder.imagepath + "/" + prod.Langs[lang].Avatar)

	// 	if len(b) > 512 {
	// 		filetype := "jpg"
	// 		filename := prod.Langs[lang].Slug + ".jpg"
	// 		for _, v := range imagesizes {
	// 			w, _ := strconv.Atoi(v)
	// 			newImage, _ := c3mcommon.ImgResize(b, uint(w), 0)
	// 			filename = prod.Langs[lang].Slug + v + "." + filetype
	// 			err := ioutil.WriteFile(imgfolder+"/"+filename, newImage, 755)
	// 			c3mcommon.CheckError("Resize file - folder:"+imgfolder+" - filename:"+filename, err)
	// 		}
	// 		//thumb
	// 		CfgItem := rptempl.GetTemplateConfigByKey(builder.bs.ShopConfigs.ShopID, builder.bs.TemplateCode, "imagethumb")
	// 		if CfgItem.Value != "" {
	// 			thumbwidth := mycrypto.Base64Decompress(CfgItem.Value)
	// 			thumbwint, _ := strconv.Atoi(thumbwidth)
	// 			newImage, _ := c3mcommon.ImgResize(b, uint(thumbwint), 0)
	// 			filename = prod.Langs[lang].Slug + "." + filetype
	// 			ioutil.WriteFile(imgfolder+"/"+filename, newImage, 755)
	// 		}
	// 	} else {
	// 		builder.logtxt += "\n" + fmt.Sprintf("image not found or invalid. imgfolder:%s - image:%s", imgfolder, builder.imagepath+"/"+prod.Langs[lang].Avatar)
	// 	}
	// 	prodjson, _ := json.Marshal(v)
	// 	json.Unmarshal(prodjson, &PData)
	// 	// pagedata
	// 	PData.Code = prod.Code

	// 	//write file index.html
	// 	var seo SeoConfig
	// 	seo.Title = v.Title
	// 	seo.Description = v.Description
	// 	seo.Lang = lang
	// 	//get cat slug:

	// 	pagename := prodcat.Langs[lang].Slug + "_page"
	// 	altpagename := "prod"
	// 	PData.Templ = pagename
	// 	PData.AltTempl = altpagename
	// 	PData.Title = PData.Name
	// 	PData.Avatar = prod.Langs[lang].Slug + "/" + prod.Langs[lang].Slug + ".jpg"
	// 	PData.CatTitle = prodcat.Langs[lang].Title
	// 	PData.CatSlug = prodcat.Langs[lang].Slug
	// 	index := builder.bottomScript(pagename, altpagename, lang)

	// 	// pagefolder := builder.webpath + "/" + v.PageInfo.Slug[:1] + "/" + v.PageInfo.Slug
	// 	// os.MkdirAll(pagefolder, 755)
	// 	// builder.logtxt+="\n"+fmt.Sprintf("create prod pagefolder:%s", pagefolder)
	// 	// fo, err := os.Create(pagefolder + "/index.html")
	// 	// c3mcommon.CheckError("error creating index.html %s", err)
	// 	// if _, err := fo.Write([]byte(index)); err != nil {
	// 	// 	c3mcommon.CheckError("error write index.html file %s", err)
	// 	// }
	// 	// fo.Close()

	// 	builder.FTPUpload("index.html", v.Slug[:1], index)

	// 	builder.logtxt += "\n" + fmt.Sprintf("%s encode", PData.Title)
	// 	databytes, err := json.Marshal(PData)
	// 	c3mcommon.CheckError("databytes json parse to string: ", err)
	// 	data["Page"] = json.RawMessage(string(databytes))
	// 	jsonbytes, err := json.Marshal(data)
	// 	c3mcommon.CheckError("jsonbytes json parse to string: ", err)
	// 	//builder.logtxt+="\n"+fmt.Sprintf("Page data %s", string(jsonbytes))
	// 	slug := v.Slug + lang + "datacache"
	// 	cachecontent := strings.Replace(lzjs.CompressToBase64(string(jsonbytes)), "=", "", -1)
	// 	builder.outputFiles("data", slug, cachecontent)

	// }

}

func (builder *Builder) buildNews() {
	// //builder.webpath := viper.GetString("config.builder.webpath") + "/" + builder.bs.ShopConfigs.ShopID

	// //build cache data
	// data := make(map[string]json.RawMessage)
	// PData := struct {
	// 	Templ    string
	// 	AltTempl string
	// 	Title    string
	// 	Slug     string
	// 	Code     string

	// 	Description string
	// 	Content     string
	// 	Avatar      string
	// 	Images      []string

	// 	Items [](struct {
	// 		Name string
	// 		Slug string
	// 	})
	// }{}

	// newsitem := rpch.GetNewsByCode(viper.GetString("config.builderid"), builder.bs.ShopConfigs.ShopID, builder.bs.ObjectId)

	// newscat := rpch.GetNewsCatByCode(viper.GetString("config.builderid"), builder.bs.ShopConfigs.ShopID, newsitem.CatID)
	// for lang, v := range newsitem.Langs {
	// 	if newsitem.Langs[lang] == nil || !newsitem.Publish {
	// 		continue
	// 	}
	// 	prodjson, _ := json.Marshal(v)
	// 	json.Unmarshal(prodjson, &PData)
	// 	// pagedata
	// 	PData.Code = newsitem.Code

	// 	//write file index.html
	// 	var seo SeoConfig
	// 	seo.Title = v.Title
	// 	seo.Description = v.Description
	// 	seo.Lang = lang
	// 	//get cat slug:

	// 	pagename := newscat.Langs[lang].Slug + "_page"
	// 	altpagename := "news"
	// 	PData.Templ = pagename
	// 	PData.AltTempl = altpagename
	// 	index := builder.bottomScript(pagename, altpagename, lang)

	// 	// pagefolder := builder.webpath + "/" + v.Slug[:1] + "/" + v.Slug
	// 	// os.MkdirAll(pagefolder, 755)
	// 	// fo, err := os.Create(pagefolder + "/index.html")
	// 	// c3mcommon.CheckError("error creating index.html %s", err)
	// 	// if _, err := fo.Write([]byte(index)); err != nil {
	// 	// 	c3mcommon.CheckError("error write index.html file %s", err)
	// 	// }
	// 	// fo.Close()

	// 	builder.FTPUpload("index.html", v.Slug[:1], index)

	// 	databytes, err := json.Marshal(PData)
	// 	c3mcommon.CheckError("databytes json parse to string: ", err)
	// 	data["Page"] = json.RawMessage(string(databytes))
	// 	jsonbytes, err := json.Marshal(data)
	// 	c3mcommon.CheckError("jsonbytes json parse to string: ", err)
	// 	//builder.logtxt+="\n"+fmt.Sprintf("Page data %s", string(jsonbytes))
	// 	slug := v.Slug + lang + "datacache"
	// 	cachecontent := strings.Replace(lzjs.CompressToBase64(string(jsonbytes)), "=", "", -1)
	// 	builder.outputFiles("data", slug, cachecontent)
	// }

}

func (builder *Builder) buildPage() {

	//build cache data
	data := make(map[string]json.RawMessage)
	PData := struct {
		Templ       string
		AltTempl    string
		Title       string
		Slug        string
		Code        string
		LangLinks   []models.LangLink
		Description string
		Content     string
		Lang        string
		PageType    string
	}{}

	var page models.Page
	err := json.Unmarshal([]byte(builder.bs.Data), &page)
	if c3mcommon.CheckError("error parse page", err) {
		for lang, v := range page.Langs {
			//clone pagedata
			if v.Title == "" {
				continue
			}
			//write file index.html
			var seo SeoConfig
			seo.Title = v.Title
			seo.Description = v.Description
			seo.Lang = lang
			pagename := page.Code
			altpagename := "page"
			PData.PageType = page.Code
			PData.Templ = pagename
			PData.AltTempl = altpagename
			PData.Lang = lang
			index := builder.bottomScript(pagename, altpagename, lang, page.Seo)

			// pagefolder := builder.webpath + "/" + builder.bs.Slug[:1] + "/" + builder.bs.Slug
			// os.MkdirAll(pagefolder, 755)
			// fo, err := os.Create(pagefolder + "/index.html")
			// c3mcommon.CheckError("error creating index.html %s", err)
			// if _, err := fo.Write([]byte(index)); err != nil {
			// 	c3mcommon.CheckError("error write index.html file %s", err)
			// }
			// fo.Close()

			//page seo description
			index = strings.Replace(index, `<meta name="description" content="">`, `<meta name="description" content="a`+page.Langs[lang].Description+`">`, 1)

			slug := ""
			if lang == builder.bs.ShopConfigs.DefaultLang && page.Code == "home" {
				builder.FTPUpload("index.html", "./", index)
				slug = "datacache"
			} else {
				builder.FTPUpload("index.html", v.Slug, index)
				slug = v.Slug + "datacache"
			}

			PData.Title = v.Title

			PData.Description = v.Description

			PData.Content = v.Content
			PData.Templ = page.Code
			if page.Code == "home" {
				PData.Templ = "index"
			}
			PData.Slug = v.Slug
			PData.LangLinks = page.LangLinks
			for i, _ := range PData.LangLinks {
				if PData.LangLinks[i].Code == "vi" {
					PData.LangLinks[i].Name = "Vietnamese"
				}
				PData.LangLinks[i].Flag = c3mcommon.Code2Flag(PData.LangLinks[i].Code)
			}
			PData.Code = page.Code
			databytes, err := json.Marshal(PData)
			c3mcommon.CheckError("PData json parse to string: ", err)
			data["Page"] = json.RawMessage(string(databytes))

			//page blocks
			datablock := make(map[string]map[string]string)
			for _, block := range page.Blocks {
				rs := make(map[string]string)
				for _, item := range block.Items {
					rs[item.Key] = item.Value[lang]
				}
				datablock[block.Name] = rs
			}
			b, err := json.Marshal(datablock)
			c3mcommon.CheckError("page blocks json parse to string: ", err)
			data["Blocks"] = json.RawMessage(string(b))

			jsonbytes, err := json.Marshal(data)
			c3mcommon.CheckError("jsonbytes json parse to string: ", err)
			//builder.logtxt+="\n"+fmt.Sprintf("Page data %s", string(jsonbytes))
			cachecontent := string(jsonbytes)
			if builder.dev != "true" {
				cachecontent = strings.Replace(lzjs.CompressToBase64(cachecontent), "=", "", -1)
			}

			builder.outputFiles("data", slug, cachecontent)
			builder.logtxt += "\n" + fmt.Sprintf("build page "+v.Slug)
		}
	}
}
