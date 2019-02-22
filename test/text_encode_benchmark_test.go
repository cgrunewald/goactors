package test

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/cgrunewald/goactors/actors"
)

type TextReaderActor struct {
	actors.DefaultActor
	filename          string
	codedText         []int32
	encoderActorRef   actors.ActorRef
	fileReaderChannel chan bool
	wg                *sync.WaitGroup
	lastSeqNum        int32
}

func (a *TextReaderActor) OnStart(context actors.ActorContext) {
	if a.encoderActorRef == nil {
		panic("Need to set the encoder actor ref on construction")
	}

	if a.filename == "" {
		panic("Need to set the filename on construction")
	}

	a.codedText = make([]int32, 0, 10000)
	a.fileReaderChannel = make(chan bool)

	selfRef := context.SelfRef()

	go (func() {
		file, err := os.Open(a.filename)
		if err != nil {
			panic("could not open filename")
		}

		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanWords)

		tokenBatch := make([]string, 0, 10)
		var seqNum int32 = 0
	loop:
		for {
			select {
			case <-a.fileReaderChannel:
				break loop
			default:
				if scanner.Scan() {
					token := scanner.Text()
					tokenBatch = append(tokenBatch, token)
					if len(tokenBatch) == cap(tokenBatch) {
						selfRef.Send(selfRef, EncodeRequest{seqNum: seqNum, tokens: tokenBatch})
						tokenBatch = make([]string, 0, 10)
						seqNum++
					}
				} else {
					if err := scanner.Err(); err == nil {
						selfRef.Send(selfRef, ReadDone{lastSeqNum: seqNum - 1})
						break loop
					} else {
						panic(err)
					}
				}
				// read the file
			}
		}

	})()
}

type ReadDone struct {
	lastSeqNum int32
}

func (a *TextReaderActor) OnStop() {
	close(a.fileReaderChannel)
}

func (a *TextReaderActor) Receive(context actors.ActorContext, message interface{}) {
	switch message.(type) {
	case EncodeRequest:
		request := message.(EncodeRequest)
		fmt.Println(request)
		a.encoderActorRef.Send(context.SelfRef(), request)
		break
	case EncodeResponse:
		response := message.(EncodeResponse)
		fmt.Println(message)

		if response.seqNum == a.lastSeqNum && a.lastSeqNum > 0 {
			a.wg.Done()
		}
		break
	case ReadDone:
		a.lastSeqNum = message.(ReadDone).lastSeqNum
		break
	}
	response, ok := message.(EncodeResponse)
	if !ok {
		return
	}

	fmt.Println(response)
}

type TextEncoderActor struct {
	actors.DefaultActor
	tokenLookup map[string]int32
	nextTokenID int32
}

type EncodeRequest struct {
	seqNum int32
	tokens []string
}

type EncodeResponse struct {
	seqNum int32
	tokens []int32
}

func (a *TextEncoderActor) OnStart(context actors.ActorContext) {
	fmt.Println("Text encoder started")
	a.tokenLookup = make(map[string]int32)
	a.nextTokenID = 0
}

func (a *TextEncoderActor) Receive(context actors.ActorContext, message interface{}) {
	fmt.Println("Received message", message)

	if request, ok := message.(EncodeRequest); ok {
		codedTokens := make([]int32, 0, len(request.tokens))
		for _, val := range request.tokens {
			if _, ok := a.tokenLookup[val]; !ok {
				a.tokenLookup[val] = a.nextTokenID
				a.nextTokenID++
			}

			codedTokens = append(codedTokens, a.tokenLookup[val])
		}

		context.SenderRef().Send(context.SelfRef(), EncodeResponse{seqNum: request.seqNum, tokens: codedTokens})
	}
}

func TestTextEncode(t *testing.T) {

	system := actors.NewSystem("encoder")
	context := system.Context()

	encoderActor := context.CreateActorFromFunc(func() actors.Actor {
		return &TextEncoderActor{}
	}, "encoder")

	wg := sync.WaitGroup{}
	wg.Add(1)
	context.CreateActorFromFunc(func() actors.Actor {
		return &TextReaderActor{
			encoderActorRef: encoderActor,
			filename:        "./odyssey.txt",
			wg:              &wg,
		}
	}, "reader")

	wg.Wait()
	context.Stop(context.SelfRef())

	system.Wait()
}
