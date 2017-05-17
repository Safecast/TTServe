// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// FTP server that enables downloading of firmware to devices for DFU
package main

import (
    "crypto/tls"
    "errors"
    "io/ioutil"
    "os"
    "fmt"
	ftp "github.com/fclairamb/ftpserver/server"
)

// Primary internal data structure
var (
    ftpServer *ftp.FtpServer
)

// FTPInboundHandler kicks off inbound messages coming from all sources, then serve HTTP
func FTPInboundHandler() {

    fmt.Printf("Now handling inbound FTP on :%d\n", TTServerFTPPort)

    ftpServer = ftp.NewFtpServer(newFtpDriver())
    err := ftpServer.ListenAndServe()
    if err != nil {
        fmt.Printf("Error listening on FTP: %s\n", err)
    }

}

// FTPStop stops the FTP server
func FTPStop() {
    ftpServer.Stop()
}

// FtpDriver defines a very basic serverftp driver
type ftpDriver struct {
    baseDir   string
    tlsConfig *tls.Config
}

// newFtpDriver creates a new instance of an FTP driver
func newFtpDriver() *ftpDriver {
    directory := SafecastDirectory()
    directory = directory + TTServerBuildPath
    driver := &ftpDriver{}
    driver.baseDir = directory
    return driver
}

func (driver *ftpDriver) WelcomeUser(cc ftp.ClientContext) (string, error) {
    cc.SetDebug(true)
    return "Welcome to TTSERVE", nil
}

func (driver *ftpDriver) AuthUser(cc ftp.ClientContext, user, pass string) (ftp.ClientHandlingDriver, error) {
    if user == "bad" || pass == "bad" {
        return nil, errors.New("bad username or password")
    }
    return driver, nil
}

func (driver *ftpDriver) GetTLSConfig() (*tls.Config, error) {
    if driver.tlsConfig == nil {
        fmt.Printf("FTP: Loading certificate\n")
        directory := SafecastDirectory()
        directory = directory + TTServerFTPCertPath
        if cert, err := tls.LoadX509KeyPair(directory+"/mycert.crt", directory+"/mycert.key"); err == nil {
            driver.tlsConfig = &tls.Config{
                NextProtos:   []string{"ftp"},
                Certificates: []tls.Certificate{cert},
            }
        } else {
            return nil, err
        }
    }
    return driver.tlsConfig, nil
}

func (driver *ftpDriver) ChangeDirectory(cc ftp.ClientContext, directory string) error {
    _, err := os.Stat(driver.baseDir + directory)
    return err
}

func (driver *ftpDriver) MakeDirectory(cc ftp.ClientContext, directory string) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("MKDIR not implemented")

    return os.Mkdir(driver.baseDir+directory, 0777)
}

func (driver *ftpDriver) ListFiles(cc ftp.ClientContext) ([]os.FileInfo, error) {

    path := driver.baseDir + cc.Path()

    files, err := ioutil.ReadDir(path)

    return files, err
}

func (driver *ftpDriver) UserLeft(cc ftp.ClientContext) {

}

func (driver *ftpDriver) OpenFile(cc ftp.ClientContext, path string, flag int) (ftp.FileStream, error) {

    path = driver.baseDir + path

    // Ftp NOT IMPLEMENTED - our FTP server is read-only root-only open-to-all
    flag = os.O_RDONLY

    // If we are writing and we are not in append mode, we should remove the file
    if 0 != (flag & os.O_WRONLY) {
        flag |= os.O_CREATE
        if 0 == (flag & os.O_APPEND) {
            os.Remove(path)
        }
    }

    return os.OpenFile(path, flag, 0666)
}

func (driver *ftpDriver) GetFileInfo(cc ftp.ClientContext, path string) (os.FileInfo, error) {
    path = driver.baseDir + path

    return os.Stat(path)
}

func (driver *ftpDriver) CanAllocate(cc ftp.ClientContext, size int) (bool, error) {
    return true, nil
}

func (driver *ftpDriver) ChmodFile(cc ftp.ClientContext, path string, mode os.FileMode) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("CHMOD not implemented")

    path = driver.baseDir + path
    return os.Chmod(path, mode)
}

func (driver *ftpDriver) DeleteFile(cc ftp.ClientContext, path string) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("RM not implemented")

    path = driver.baseDir + path
    return os.Remove(path)
}

func (driver *ftpDriver) RenameFile(cc ftp.ClientContext, from, to string) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("MV not implemented")

    from = driver.baseDir + from
    to = driver.baseDir + to
    return os.Rename(from, to)
}

func (driver *ftpDriver) GetSettings() *ftp.Settings {
    config := &ftp.Settings{}
	config.PublicHost = ThisServerAddressIPv4
    config.ListenHost = ""
    config.ListenPort = TTServerFTPPort
    config.MaxConnections = 10000
    return config
}
