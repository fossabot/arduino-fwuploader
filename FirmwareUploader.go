/*
  FirmwareUploader.go - A firmware uploader for the WiFi101 module.
  Copyright (c) 2015 Arduino LLC.  All right reserved.

  This library is free software; you can redistribute it and/or
  modify it under the terms of the GNU Lesser General Public
  License as published by the Free Software Foundation; either
  version 2.1 of the License, or (at your option) any later version.

  This library is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
  Lesser General Public License for more details.

  You should have received a copy of the GNU Lesser General Public
  License along with this library; if not, write to the Free Software
  Foundation, Inc., 51 Franklin St, Fifth Floor, Boston, MA  02110-1301  USA
*/
package main

import (
	"bytes"
	"certificates"
	"errors"
	"flag"
	"flasher"
	"io/ioutil"
	"log"
)

var portName string
var rootCertDir string
var firmwareFile string

var f *flasher.Flasher
var payloadSize uint16

func init() {
	flag.StringVar(&portName, "port", "", "serial port to use for flashing")
	flag.StringVar(&rootCertDir, "certs", "", "root certificate directory")
	flag.StringVar(&firmwareFile, "firmware", "", "firmware file to flash")
}

func main() {
	flag.Parse()

	var err error
	f, err = flasher.Open(portName)
	if err != nil {
		log.Fatal(err)
	}

	// Synchronize with programmer
	if err := f.Hello(); err != nil {
		log.Fatal(err)
	}

	// Check maximum supported payload size
	payloadSize, err = f.GetMaximumPayloadSize()
	if err != nil {
		log.Fatal(err)
	}
	if payloadSize < 1024 {
		log.Fatalf("Programmer reports %d as maximum payload size (1024 is needed)", payloadSize)
	}

	if rootCertDir != "" {
		log.Printf("Converting and flashing certificates from '%v'", rootCertDir)
		if err := flashCerts(); err != nil {
			log.Fatal(err)
		}
	}

	if firmwareFile != "" {
		log.Printf("Flashing firmware from '%v'", firmwareFile)
		if err := flashCerts(); err != nil {
			log.Fatal(err)
		}
	}

	f.Close()
}

func flashCerts() error {
	CERTIFICATES_OFFSET := 0x4000

	certificatesData, err := certificates.Convert(rootCertDir)
	if err != nil {
		return err
	}

	return flashChunk(CERTIFICATES_OFFSET, certificatesData)
}

func flashFirmware() error {
	FIRMWARE_OFFSET := 0x6800
	FIRWARE_LENGTH := 0x30800

	fwData, err := ioutil.ReadFile(firmwareFile)
	if err != nil {
		return err
	}

	return flashChunk(FIRMWARE_OFFSET, fwData[FIRMWARE_OFFSET:(FIRMWARE_OFFSET+FIRWARE_LENGTH)])
}

func flashChunk(offset int, buffer []byte) error {
	chunkSize := int(payloadSize)
	bufferLength := len(buffer)

	if err := f.Erase(uint32(offset), uint32(bufferLength)); err != nil {
		return err
	}

	for i := 0; i < bufferLength; i += chunkSize {
		start := i
		end := i + chunkSize
		if end > bufferLength {
			end = bufferLength
		}
		if err := f.Write(uint32(offset+i), buffer[start:end]); err != nil {
			return err
		}
	}

	var flashData []byte
	for i := 0; i < bufferLength; i += chunkSize {
		readLength := chunkSize
		if (i + chunkSize) > bufferLength {
			readLength = bufferLength % chunkSize
		}

		data, err := f.Read(uint32(offset+i), uint32(readLength))
		if err != nil {
			return err
		}

		flashData = append(flashData, data...)
	}

	if !bytes.Equal(buffer, flashData) {
		return errors.New("Flash data does not match written!")
	}

	return nil
}
