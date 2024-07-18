package main

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

type SSH struct {
	client *ssh.Client
}

func NewSSH(host string) (*SSH, error) {
	keyStr, err := ReadStringFromFile("C:/Users/User/.ssh/id_ed25519")
	if err != nil {
		return nil, fmt.Errorf("couldn't read private key: %w", err)
	}
	key, err := ssh.ParsePrivateKey([]byte(keyStr))
	if err != nil {
		return nil, fmt.Errorf("couldn't parse private key: %w", err)
	}
	config := &ssh.ClientConfig{
		User:            "maf",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("couldn't dial ssh: %w", err)
	}
	return &SSH{client: client}, nil
}

func (ssh *SSH) Exec(command string) (output string, err error) {
	session, err := ssh.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("couldn't create SSH session: %w", err)
	}
	defer session.Close()
	outputBytes, err := session.CombinedOutput(command)
	if err != nil {
		return string(outputBytes), fmt.Errorf("couldn't run command: %w", err)
	}
	return string(outputBytes), nil
}

func (ssh *SSH) Close() {
	ssh.client.Close()
}
