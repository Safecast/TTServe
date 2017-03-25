// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

type TTServeConfig struct {

	// Access key for TTN
	TtnAppAccessKey		string		`json:"ttn_app_access_key,omitempty"`

	// Slack integration info
	SlackChannels		string		`json:"slack_channels,omitempty"`
	SlackInboundTokens	string		`json:"slack_inbound_tokens,omitempty"`
	SlackOutboundUrls	string		`json:"slack_outbound_urls,omitempty"`

}
