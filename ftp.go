// FTP server for downloading code to the devices
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

// Kick off inbound messages coming from all sources, then serve HTTP
func FtpInboundHandler() {

    fmt.Printf("Now handling inbound FTP on %s:%d\n", TTServerFTPAddress, TTServerFTPPort)

    ftpServer = ftp.NewFtpServer(NewFtpDriver())
    err := ftpServer.ListenAndServe()
    if err != nil {
        fmt.Printf("Error listening on FTP: %s\n", err)
    }

}

// Stop the FTP server
func FtpStop() {
    ftpServer.Stop()
}

// FtpDriver defines a very basic serverftp driver
type FtpDriver struct {
    baseDir   string
    tlsConfig *tls.Config
}

// Create a new instance of an FTP driver
func NewFtpDriver() *FtpDriver {
    directory := SafecastDirectory()
    directory = directory + TTServerBuildPath
    driver := &FtpDriver{}
    driver.baseDir = directory
    return driver
}

func (driver *FtpDriver) WelcomeUser(cc ftp.ClientContext) (string, error) {
    cc.SetDebug(true)
    return "Welcome to TTSERVE", nil
}

func (driver *FtpDriver) AuthUser(cc ftp.ClientContext, user, pass string) (ftp.ClientHandlingDriver, error) {
    if user == "bad" || pass == "bad" {
        return nil, errors.New("BAD username or password !")
    } else {
        return driver, nil
    }
}

func (driver *FtpDriver) GetTLSConfig() (*tls.Config, error) {
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

func (driver *FtpDriver) ChangeDirectory(cc ftp.ClientContext, directory string) error {
    _, err := os.Stat(driver.baseDir + directory)
    return err
}

func (driver *FtpDriver) MakeDirectory(cc ftp.ClientContext, directory string) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("MKDIR not implemented")

    return os.Mkdir(driver.baseDir+directory, 0777)
}

func (driver *FtpDriver) ListFiles(cc ftp.ClientContext) ([]os.FileInfo, error) {

    path := driver.baseDir + cc.Path()

    files, err := ioutil.ReadDir(path)

    return files, err
}

func (driver *FtpDriver) UserLeft(cc ftp.ClientContext) {

}

func (driver *FtpDriver) OpenFile(cc ftp.ClientContext, path string, flag int) (ftp.FileStream, error) {

    path = driver.baseDir + path

    // Ftp NOT IMPLEMENTED - our FTP server is read-only root-only open-to-all
    flag = os.O_RDONLY

    // If we are writing and we are not in append mode, we should remove the file
    if (flag & os.O_WRONLY) != 0 {
        flag |= os.O_CREATE
        if (flag & os.O_APPEND) == 0 {
            os.Remove(path)
        }
    }

    return os.OpenFile(path, flag, 0666)
}

func (driver *FtpDriver) GetFileInfo(cc ftp.ClientContext, path string) (os.FileInfo, error) {
    path = driver.baseDir + path

    return os.Stat(path)
}

func (driver *FtpDriver) CanAllocate(cc ftp.ClientContext, size int) (bool, error) {
    return true, nil
}

func (driver *FtpDriver) ChmodFile(cc ftp.ClientContext, path string, mode os.FileMode) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("CHMOD not implemented")

    path = driver.baseDir + path
    return os.Chmod(path, mode)
}

func (driver *FtpDriver) DeleteFile(cc ftp.ClientContext, path string) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("RM not implemented")

    path = driver.baseDir + path
    return os.Remove(path)
}

func (driver *FtpDriver) RenameFile(cc ftp.ClientContext, from, to string) error {

    // Ftp NOT IMPLEMENTED, because our FTP server is read-only root-only open-to-all
    return errors.New("MV not implemented")

    from = driver.baseDir + from
    to = driver.baseDir + to
    return os.Rename(from, to)
}

func (driver *FtpDriver) GetSettings() *ftp.Settings {
    config := &ftp.Settings{}
	config.PublicHost = ThisServerAddressIPv4
    config.ListenHost = ""
    config.ListenPort = TTServerFTPPort
    config.MaxConnections = 10000
    return config
}
