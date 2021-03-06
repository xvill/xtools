package xutil

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ftp4go "github.com/shenshouer/ftp4go"
)

type XFtp struct {
	Addr            string
	User            string
	Pwd             string
	PASV            string
	FilePattern     string
	LocalFilePrefix string
	Conn            *ftp4go.FTP
}

func (c *XFtp) Connect() (err error) {
	addra := strings.Split(c.Addr, ":")
	host := addra[0]
	port, _ := strconv.Atoi(addra[1])

	c.Conn = ftp4go.NewFTP(0) // 1 for debugging
	_, err = c.Conn.Connect(host, port, "")
	if err != nil {
		return err
	}

	_, err = c.Conn.Login(c.User, c.Pwd, "")
	if err != nil {
		return err
	}
	if c.PASV == "PORT" {
		c.Conn.SetPassive(false)
	}
	return nil
}

func (c XFtp) MKdir(path string) {
	xdir, xfile := filepath.Split(path)
	fname := filepath.Join(xdir, xfile)
	xdirFiles, _ := c.Conn.Nlst(xdir)
	for _, v := range xdirFiles {
		if v == fname {
			return
		}
	}
	_, err := c.Conn.Mkd(path)
	if err != nil {
		c.MKdir(xdir)
		c.MKdir(fname)
	}
	return
}

func (c XFtp) NameList() (ftpfiles []string) {
	xdir, xfile := filepath.Split(c.FilePattern)
	files, err := c.Conn.Nlst(xdir)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	for _, v := range files {
		v = filepath.Base(v)
		if ok, _ := filepath.Match(xfile, v); ok {
			ftpfiles = append(ftpfiles, filepath.Join(xdir, v))
		}
	}
	return ftpfiles
}

func (c XFtp) DownloadFiles(files []string) (dat map[string]string, err error) {
	dat = make(map[string]string, 0)
	if len(files) == 0 {
		return
	}
	if c.LocalFilePrefix != "" {
		err = IsDirsExist([]string{c.LocalFilePrefix}, false)
		if err == nil {
			c.LocalFilePrefix = filepath.Dir(c.LocalFilePrefix+string(filepath.Separator)) + string(filepath.Separator)
		} else {
			return dat, err
		}
	}

	fmt.Println("DownloadFiles begin")
	for _, file := range files {
		if c.LocalFilePrefix == "" {
			c.LocalFilePrefix = time.Now().Format("20060102150405") + "_"
		}
		localpath := c.LocalFilePrefix + filepath.Base(file)
		fmt.Println("DownloadFile " + file + " to " + localpath)
		err = c.Conn.DownloadFile(file, localpath, false)
		if err != nil {
			return dat, err
		}
		dat[file] = localpath
	}
	fmt.Println("DownloadFiles end")
	return dat, nil
}

func (c XFtp) Logout() error {
	_, err := c.Conn.Quit()
	return err
}

func (c *XFtp) ConnectAndDownload() (files map[string]string, err error) {
	err = c.Connect()
	if err != nil {
		return nil, err
	}
	defer c.Logout()
	files, err = c.DownloadFiles(c.NameList())
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (c *XFtp) UploadFiles(files map[string]string, useLineMode bool) (retInfo map[string]error) {
	retInfo = make(map[string]error, 0)
	for fname, tname := range files {
		retInfo[fname] = c.Conn.UploadFile(tname, fname, useLineMode, nil)
	}
	return
}

//GetFTPFiles 获取 FTP/SFTP 匹配的文件
func GetFTPFiles(ftptype, addr, user, pwd, pasv, pattern, localfileprefix string, expectfiles []string) (files map[string]string, err error) {

	ftpfiles := make([]string, 0)
	var xftp XFtp
	var xsftp XSFtp
	files = make(map[string]string, 0)

	switch ftptype {
	case "FTP":
		xftp = XFtp{Addr: addr,
			User:            user,
			Pwd:             pwd,
			PASV:            pasv,
			FilePattern:     pattern,
			LocalFilePrefix: localfileprefix}

		log.Println(xftp.FilePattern, filepath.Dir(xftp.FilePattern))
		err = xftp.Connect()
		if err != nil {
			log.Println(err)
			return
		}
		defer xftp.Logout()
		ftpfiles = xftp.NameList()
	case "SFTP":
		xsftp = XSFtp{Addr: addr,
			User:            user,
			Pwd:             pwd,
			FilePattern:     pattern,
			LocalFilePrefix: localfileprefix}

		log.Println(xsftp.FilePattern, filepath.Dir(xsftp.FilePattern))
		err = xsftp.Connect()
		if err != nil {
			log.Println(err)
			return
		}
		defer xsftp.Logout()
		ftpfiles = xsftp.NameList()
	}
	// ------------------------------------------------------------------------------
	for i := range ftpfiles {
		ftpfiles[i] = fmt.Sprintf("[%s]%s", addr, ftpfiles[i])
	}

	getftpfiles := StringsMinus(ftpfiles, expectfiles) // 要下载的文件 = FTP文件名 - 已入库的文件
	if len(getftpfiles) == 0 {                         //没有可下载的文件
		return
	}

	for i := range getftpfiles {
		getftpfiles[i] = strings.TrimPrefix(getftpfiles[i], "["+addr+"]")
	}
	// ------------------------------------------------------------------------------
	xfiles := make(map[string]string, 0)
	switch ftptype {
	case "FTP":
		xfiles, err = xftp.DownloadFiles(getftpfiles)
	case "SFTP":
		xfiles, err = xsftp.DownloadFiles(getftpfiles)
	}

	for ftpfile, localfile := range xfiles {
		files[fmt.Sprintf("[%s]%s", addr, ftpfile)] = localfile
	}
	return
}
