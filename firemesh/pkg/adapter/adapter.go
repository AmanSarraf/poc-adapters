/*
Copyright 2022 TriggerMesh Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package FIREMESH implements a CloudEvents adapter that allows Triggermesh Sources
// to integrate with a Firefly Node.
package firemesh

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	pkgadapter "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"

	targetce "github.com/triggermesh/triggermesh/pkg/targets/adapter/cloudevents"
)

type BroadcastMessageData struct {
	Value interface{} `json:"value"`
}

type BroadcastBlobData struct {
	Data   []Data                 `json:"data"`
	Header BroadcastMessageHeader `json:"header"`
}

type Data struct {
	ID string `json:"id"`
}

type BroadcastMessageHeader struct {
	Tag    string   `json:"tag"`
	Topics []string `json:"topics"`
}

type BroadcastMessage struct {
	Header BroadcastMessageHeader `json:"header"`
	Data   []BroadcastMessageData `json:"data"`
}

type BlobDataUploadResponse struct {
	ID        string      `json:"id"`
	Validator string      `json:"validator"`
	Namespace string      `json:"namespace"`
	Hash      string      `json:"hash"`
	Created   time.Time   `json:"created"`
	Value     interface{} `json:"value"`
	Blob      Blob        `json:"blob"`
}

type Blob struct {
	Hash string `json:"hash"`
	Size int    `json:"size"`
	Name string `json:"name"`
}

// EnvAccessorCtor for configuration parameters
func EnvAccessorCtor() pkgadapter.EnvConfigAccessor {
	return &envAccessor{}
}

type envAccessor struct {
	pkgadapter.EnvConfig

	FF     string   `envconfig:"FF" required:"true"`
	Topics []string `envconfig:"TOPIC" required:"false`

	// BridgeIdentifier is the name of the bridge workflow this target is part of
	BridgeIdentifier string `envconfig:"EVENTS_BRIDGE_IDENTIFIER"`
	// CloudEvents responses parametrization
	CloudEventPayloadPolicy string `envconfig:"EVENTS_PAYLOAD_POLICY" default:"error"`
	// Sink defines the target sink for the events. If no Sink is defined the
	// events are replied back to the sender.
	Sink string `envconfig:"K_SINK"`
}

// NewAdapter adapter implementation
func NewAdapter(ctx context.Context, envAcc pkgadapter.EnvConfigAccessor, ceClient cloudevents.Client) pkgadapter.Adapter {
	env := envAcc.(*envAccessor)
	logger := logging.FromContext(ctx)

	replier, err := targetce.New(env.Component, logger.Named("replier"),
		targetce.ReplierWithStatefulHeaders(env.BridgeIdentifier),
		targetce.ReplierWithStaticResponseType("io.triggermesh.firemesh.error"),
		targetce.ReplierWithPayloadPolicy(targetce.PayloadPolicy(env.CloudEventPayloadPolicy)))
	if err != nil {
		logger.Panicf("Error creating CloudEvents replier: %v", err)
	}

	return &firemeshadapter{
		ff:       env.FF,
		topic:    env.Topics,
		sink:     env.Sink,
		replier:  replier,
		ceClient: ceClient,
		logger:   logger,
	}
}

var _ pkgadapter.Adapter = (*firemeshadapter)(nil)

type firemeshadapter struct {
	ff       string
	topic    []string
	sink     string
	replier  *targetce.Replier
	ceClient cloudevents.Client
	logger   *zap.SugaredLogger
}

// Start is a blocking function and will return if an error occurs
// or the context is cancelled.
func (a *firemeshadapter) Start(ctx context.Context) error {
	a.logger.Info("Starting FIREMESH Adapter")
	return a.ceClient.StartReceiver(ctx, a.dispatch)
}

func (a *firemeshadapter) dispatch(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
	if err := a.broadcastCE(event); err != nil {
		return a.replier.Error(&event, targetce.ErrorCodeAdapterProcess, err, "failed to broadcast event")
	}

	return &event, cloudevents.ResultACK
}

// https://hyperledger.github.io/firefly/tutorials/broadcast_data.html
func (a *firemeshadapter) broadcastCE(ev cloudevents.Event) error {
	url := a.ff + "/api/v1/namespaces/default/messages/broadcast"
	method := "POST"
	client := &http.Client{}

	// If a topic has been specified, use it. Otherwise, use the incoming event type
	var topics []string
	if len(a.topic) > 0 {
		topics = append(topics, ev.Type())
	} else {
		topics = a.topic
	}

	bm := BroadcastMessage{Data: []BroadcastMessageData{BroadcastMessageData{Value: ev.Data()}}, Header: BroadcastMessageHeader{Tag: ev.Source(), Topics: topics}}
	b, err := json.Marshal(bm)
	if err != nil {
		return err
	}

	ior := bytes.NewBuffer(b)
	req, err := http.NewRequest(method, url, ior)
	if err != nil {
		return err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("content-type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	a.logger.Infof("broadcasted ce", string(body))
	return nil
}
