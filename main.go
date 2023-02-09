/**
* @program: user
*
* @description:
*
* @author: lemo
*
* @create: 2023-02-09 14:57
**/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var kubeTmp = `apiVersion: v1
clusters:
- name: default
  cluster:
    certificate-authority-data: DATA+OMITTED
    server: URL+OMITTED

contexts:
- name: default
  context:
    cluster: default
    user: default

current-context: default
kind: Config
preferences: {}

users:
- name: default
  user:
    client-certificate-data: DATA+OMITTED
    client-key-data: DATA+OMITTED`

func GetArgs(flag ...string) string {
	var args = os.Args[1:]
	for i := 0; i < len(args); i++ {
		for j := 0; j < len(flag); j++ {
			if args[i] == flag[j] {
				if i+1 < len(args) {
					return args[i+1]
				}
			}
		}
	}
	return ""
}

func FindKubeConfig() string {
	var kubeConfig = os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		var home, err = os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		kubeConfig = filepath.Join(home, ".kube", "config")
	}
	return kubeConfig
}

func ReadFromPath(path string) (string, error) {
	var absPath, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	file, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	all, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(all), nil
}

func CreateFileFromString(path string, content string) error {
	var absPath, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	file, err := os.Create(absPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = file.WriteString(content)
	if err != nil {
		return err
	}
	return nil
}

func Command(command string) *exec.Cmd {
	var cmd = exec.Command(os.Getenv("SHELL"), "-c", command)
	return cmd
}

func GenRasPrivateKey(bit int) (string, error) {
	var cmd = Command(fmt.Sprintf("openssl genrsa %d", bit))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return "", errors.New(stderr.String())
	}

	return stdout.String(), nil
}

func CreateCertSigningRequest(userName string, groupName string, privateKey string) (string, error) {
	var cmd = Command(fmt.Sprintf(`openssl req -new -key <(echo "%s") -subj "/emailAddress=%s/C=CN/ST=SD/L=JN/O=%s/OU=system/CN=%s"`,
		privateKey, userName, groupName, userName,
	))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return "", errors.New(stderr.String())
	}

	return stdout.String(), nil
}

func CreateCertWithCa(csr string, ca string, caKey string) (string, error) {
	// CA serial CA create serial
	// not necessary in macOS
	var cmd = Command(fmt.Sprintf(`openssl x509 -req -in <(echo "%s") -CA <(echo "%s") -CAkey <(echo "%s") -CAserial 475FF44D0E5C3E7803AF3B175416E30FE618B85D.srl -CAcreateserial -days 3650`,
		csr, ca, caKey,
	))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return "", errors.New(stderr.String())
	}

	_ = os.Remove("475FF44D0E5C3E7803AF3B175416E30FE618B85D.srl")

	return stdout.String(), nil
}

func SetClusterCert(confPath string, serverCa string, url string) (string, error) {
	var cmd = Command(fmt.Sprintf(`kubectl config set-cluster default --server=%s --certificate-authority <(echo "%s") --embed-certs=true --kubeconfig=%s`,
		url, serverCa, confPath,
	))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return "", errors.New(stderr.String())
	}

	return stdout.String(), nil
}

func SetCredentials(confPath string, clientCa string, clientCaKey string) (string, error) {
	var cmd = Command(fmt.Sprintf(`kubectl config set-credentials default --client-certificate <(echo "%s") --client-key <(echo "%s") --embed-certs=true --kubeconfig=%s`,
		clientCa, clientCaKey, confPath,
	))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return "", errors.New(stderr.String())
	}

	return stdout.String(), nil
}

func main() {

	var pwd, _ = os.Getwd()

	var userName = GetArgs("-u", "--user")
	var group = GetArgs("-g", "--group")
	var out = GetArgs("-o", "--out")
	var url = GetArgs("-url", "--url")
	var serverCaPath = GetArgs("-sca", "--serverCA")
	var clientCaPath = GetArgs("-ca", "--clientCA")
	var clientCaKeyPath = GetArgs("-caKey", "--clientCAKey")
	var kubeConfig = GetArgs("-kubeconfig")

	if clientCaPath == "" {
		fmt.Println("client ca not found")
		return
	}

	if clientCaKeyPath == "" {
		fmt.Println("client ca key not found")
		return
	}

	if userName == "" {
		fmt.Println("user name is empty")
		return
	}

	if group == "" {
		group = "user-group"
	}

	if out == "" {
		out = "kubeconfig"
	}

	// if kubeConfig == "" {
	// 	kubeConfig = FindKubeConfig()
	// }

	if kubeConfig == "" {
		if serverCaPath == "" {
			fmt.Println("you need provide server ca or kubeconfig")
			return
		}
		if url == "" {
			fmt.Println("you need provide server url")
			return
		}
	}

	var clientCa, err = ReadFromPath(clientCaPath)
	if err != nil {
		panic(err)
	}

	clientCaKey, err := ReadFromPath(clientCaKeyPath)
	if err != nil {
		panic(err)
	}

	privateKey, err := GenRasPrivateKey(2048)
	if err != nil {
		panic(err)
	}

	request, err := CreateCertSigningRequest(userName, group, privateKey)
	if err != nil {
		panic(err)
	}

	cert, err := CreateCertWithCa(request, clientCa, clientCaKey)
	if err != nil {
		panic(err)
	}

	if kubeConfig == "" {
		err = CreateFileFromString(out, kubeTmp)
		if err != nil {
			panic(err)
		}

		serverCa, err := ReadFromPath(serverCaPath)
		if err != nil {
			panic(err)
		}

		_, err = SetClusterCert(out, serverCa, "https://47.242.248.165:6443")
		if err != nil {
			panic(err)
		}
	} else {
		var old, err = ReadFromPath(kubeConfig)
		if err != nil {
			panic(err)
		}
		err = CreateFileFromString(out, old)
		if err != nil {
			panic(err)
		}
	}

	_, err = SetCredentials(out, cert, privateKey)
	if err != nil {
		panic(err)
	}

	fmt.Printf("user name: %s\n", userName)
	fmt.Printf("group name: %s\n", group)
	fmt.Printf("out path: %s\n", filepath.Join(pwd, out))
	fmt.Printf("test: kubectl --kubeconfig=%s auth can-i list pods\n", filepath.Join(pwd, out))
}
