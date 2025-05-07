// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

// TTServeConfig is the service configuration file format
type TTServeConfig struct {

	// TTN
	TtnAppAccessKey string `json:"ttn_app_access_key,omitempty"`

	// Slack
	SlackChannels      string `json:"slack_channels,omitempty"`
	SlackInboundTokens string `json:"slack_inbound_tokens,omitempty"`
	SlackOutboundUrls  string `json:"slack_outbound_urls,omitempty"`

	// AWS IOT MQTT Broker
	BrokerHost     string `json:"broker_host,omitempty"`
	BrokerUsername string `json:"broker_username,omitempty"`
	BrokerPassword string `json:"broker_password,omitempty"`

	// Notehub URL
	NotehubURL   string `json:"notehub_url,omitempty"`
	NotehubToken string `json:"notehub_token,omitempty"`

	// Database options
	UseExternalEndpoints bool `json:"use_external_endpoints,omitempty"`
}
