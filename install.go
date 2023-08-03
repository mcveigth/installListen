package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

func extractFirstIPFromFile(filePath string) (string, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}

	ipPattern := `\b(?:\d{1,3}\.){3}\d{1,3}\b`

	re := regexp.MustCompile(ipPattern)
	matches := re.FindAllString(string(data), -1)

	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", fmt.Errorf("no IP address found in the file")
}

func updateIPInInterfaceFile(filePath, oldIP, newIP string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	fileContent := string(content)

	updatedContent := strings.Replace(fileContent, oldIP, newIP, -1)

	err = ioutil.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func sendFile(ip, filename, destPath string, settings *ssh.ClientConfig) error {

	var ipAddr = ip + ":22"
	client, err := ssh.Dial("tcp", ipAddr, settings)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to to create SSH session: %v", err)
	}
	defer session.Close()

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()
	stat, _ := file.Stat()

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		hostIn, _ := session.StdinPipe()
		defer hostIn.Close()
		fmt.Fprintf(hostIn, "C0064 %d %s\n", stat.Size(), filename)
		io.Copy(hostIn, file)
		fmt.Fprint(hostIn, "\x00")
		wg.Done()
	}()

	var run string = "/usr/bin/scp -t " + destPath
	session.Run(run)

	return nil
}

func enableService(ip, serviceName string, settings *ssh.ClientConfig) error {
	//establish connection
	var ipAddr = ip + ":22"
	client, err := ssh.Dial("tcp", ipAddr, settings)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	var cmd string = fmt.Sprintf("systemctl daemon-reload && systemctl enable %s.service && systemctl start %s.service && systemctl status %s.service", serviceName, serviceName, serviceName)
	fmt.Println(cmd)
	var buff bytes.Buffer
	session.Stdout = &buff
	if err := session.Run(cmd); err != nil {
		log.Fatal(err)
	}
	fmt.Println(buff.String())

	return nil
}

func runCMD(ip, command string, settings *ssh.ClientConfig) error {
	//establish connection
	var ipAddr = ip + ":22"
	client, err := ssh.Dial("tcp", ipAddr, settings)
	if err != nil {
		return fmt.Errorf("failed to establish SSH connection: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	var cmd string = command
	fmt.Println(cmd)
	var buff bytes.Buffer
	session.Stdout = &buff
	if err := session.Run(cmd); err != nil {
		log.Fatal(err)
	}
	fmt.Println(buff.String())

	return nil
}

func main() {

	var uname string
	var pass string
	var currentIdentifier string
	hFlag := flag.Bool("h", false, "Prints help")

	flag.StringVar(&uname, "u", "root", "Specify username. Default is root")
	flag.StringVar(&pass, "p", "3dscan", "Specify username. Default is 3dscan")
	flag.StringVar(&currentIdentifier, "c", "124", "Specify current host identifier. Default is 124")
	flag.Parse()

	if *hFlag {
		fmt.Println(`Usage: install [OPTIONS] 

Example: install -u <username> -p <password> -c <Current Host Identifier>

Description:
The following command facilitates the installation of the listencpp camera client on a Raspberry Pi.
Parameters:
  <uname> 				Name of host
  <password> 			Password of host
  <hostIdentifier>      The identifier of the target server where the IP address should be set.

Options:
  -u string
        Specify the username for SSH authentication. Default is "root".
  -p string
        Specify the password for SSH authentication. Default is "3dscan".
  -n string
        Specify the host identifier for the target server. Default is "101".
  -c string
        Specify the current host identifier. Default is "124".
  -help, -h
        Prints help.

Note:
Make sure you have the necessary permissions to access the server, and backup any critical files before using this command.

Warning:
Incorrect usage of this command could lead .service error.

Example:
To install listencpp to the IP address 192.168.99.125, run:
$ install -u myusername -p mypassword -c 125`)
		os.Exit(0)
	}
	var sshConfig = &ssh.ClientConfig{
		User: uname,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	var raspIP string = "192.168.99." + currentIdentifier

	if err := sendFile(raspIP, "listen", "/camsoft", sshConfig); err != nil {
		log.Fatal(err)
	}

	if err := sendFile(raspIP, "listencpp.service", "/etc/systemd/system/", sshConfig); err != nil {
		log.Fatal(err)
	}

	if err := runCMD(raspIP, "chmod +x /camsoft/listen", sshConfig); err != nil {
		log.Fatal(err)
	}

	if err := enableService(raspIP, "listencpp", sshConfig); err != nil {
		log.Fatal(err)
	}

	fmt.Println("service installed and set")
}
