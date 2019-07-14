package handler

import (
	"FileStore-Server/db"
	"FileStore-Server/meta"
	"FileStore-Server/util"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

//UploadHandler: 文件上传路由
func UploadHandler(w http.ResponseWriter,r *http.Request)  {
	if r.Method == "GET" {
		//返回上传文件的html页面
		data,err := ioutil.ReadFile("./static/view/index.html")

		if err != nil {
			io.WriteString(w,fmt.Sprint("internel server error: err: %s",err.Error()))
			return
		}
		io.WriteString(w,string(data))
	}else if r.Method == "POST" {
		//接收文件流及存储到本地目录

		//获取表单上传的文件，并打开
		file,head,err := r.FormFile("file")
		defer file.Close()
		if err != nil {
			fmt.Printf("Failed to get data, err: %s\n",err.Error())
			return
		}

		//创建文件元信息实例
		fileMeta := meta.FileMeta{
			FileName:head.Filename,
			Location:"./static/tempFiles/"+head.Filename,
			UploadAt:time.Now().Format("2006-01-02 15:04:05"),
		}

		//创建本地文件
		localFile,err := os.Create(fileMeta.Location)
		defer localFile.Close()
		if err != nil {
			fmt.Printf("Failed to create file, err: %s\n",err.Error())
			return
		}

		//复制文件信息到本地文件
		fileMeta.FileSize,err = io.Copy(localFile,file)
		if err != nil {
			fmt.Printf("Failed to save data into file, err: %s\n",err.Error())
			return
		}

		//计算文件哈希值
		localFile.Seek(0,0)
		fileMeta.FileSha1 = util.FileSha1(localFile)
		//将文件元信息添加到mysql中
		meta.UpdateFileMetaDB(fileMeta)

		//更新用户文件表记录
		r.ParseForm()
		username := r.Form.Get("username")
		ok := db.OnUserFileUploadFinished(username,fileMeta.FileSha1,fileMeta.FileName,fileMeta.FileSize)
		if ok {
			http.Redirect(w,r,"/static/view/home.html",http.StatusFound)
		}else {
			w.Write([]byte("Upload Failed"))
		}

		//log
		fmt.Printf("Your file's meta is: %s\n",fileMeta)

		//重定向至上传成功页面
		http.Redirect(w,r,"/file/upload/suc",http.StatusFound)
	}
}

//UploadSucHandler: 上传成功
func UploadSucHandler(w http.ResponseWriter,r *http.Request)  {
	io.WriteString(w,"Upload finished!")
}

//GetFileMetaHandler: 通过文件hash值，获取文件元信息
func GetFileMetaHandler(w http.ResponseWriter,r *http.Request)  {
	r.ParseForm()

	//获取hash值，并通过其查询文件元信息
	filehash := r.Form["filehash"][0]
	fMeta,err := meta.GetFileMetaDB(filehash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//json格式化meta实例
	data,err := json.Marshal(fMeta)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

//DownloadHandler: 根据文件哈希值下载文件
func DownloadHandler(w http.ResponseWriter,r *http.Request)  {
	r.ParseForm()

	//获取文件hash值，并获取元信息
	fileSha1 := r.Form.Get("filehash")
	fm,err := meta.GetFileMetaDB(fileSha1)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//根据文件元信息打开本地文件
	f,err := os.Open(fm.Location)
	defer f.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//将本地文件读入内存
	data,err := ioutil.ReadAll(f)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	//设置响应头，写入数据
	w.Header().Set("Content-Type","application/octect-stream")
	w.Header().Set("content-disposition","attachment;filename=\""+fm.FileName+"\"")
	w.Write(data)
}

//FileMetaUpdateHandler: 更新文件元信息
func FileMetaUpdateHandler(w http.ResponseWriter,r *http.Request) {
	r.ParseForm()

	opType := r.Form.Get("op")
	fileSha1 := r.Form.Get("filehash")
	newFileName := r.Form.Get("filename")
	
	if opType != "0" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	curFileMeta := meta.GetFileMeta(fileSha1)
	curFileMeta.FileName = newFileName
	meta.UpdateFileMeta(curFileMeta)

	w.WriteHeader(http.StatusOK)
	data,err := json.Marshal(curFileMeta)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

//FileDeleteHandler: 删除文件
func FileDeleteHandler(w http.ResponseWriter,r *http.Request) {
	r.ParseForm()
	fileSha1 := r.Form.Get("filehash")

	//删除本地文件
	fMeta := meta.GetFileMeta(fileSha1)
	err := os.Remove(fMeta.Location)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//删除元信息
	meta.RemoveFileMeta(fileSha1)

	w.WriteHeader(http.StatusOK)
}