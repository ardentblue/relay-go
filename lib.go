package relaygo

import (
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Response timeout in seconds.
const RES_TIMEOUT = 60

type RelayDevice struct {
	sendC          chan []byte
	recvC          chan []byte
	stopC          chan struct{}
	msgChanMu      sync.RWMutex
	msgChanMap     map[string]chan resMsg
	terminateOnce  sync.Once
	isTerminated   bool
	isTerminatedMu sync.RWMutex
	ID             string
}

func NewRelayDevice(sendC chan []byte, recvC chan []byte, stopC chan struct{}) (*RelayDevice, error) {
	d := &RelayDevice{
		sendC:        sendC,
		recvC:        recvC,
		stopC:        stopC,
		msgChanMap:   make(map[string]chan resMsg),
		isTerminated: false,
	}

	go d.recvMsg()

	id, err := d.getID()
	d.ID = id

	return d, err
}

func (d *RelayDevice) getID() (string, error) {
	reqID := makeID()
	msg, err := deviceInfoMsg(reqID, "id", false)
	if err != nil {
		return "", err
	}

	res, err := d.sendMsg(reqID, msg)
	if err != nil {
		return "", err
	}

	return res.ID, nil
}

func (d *RelayDevice) GetBattery() (int64, error) {
	reqID := makeID()
	msg, err := deviceInfoMsg(reqID, "battery", false)
	if err != nil {
		return 0, err
	}

	res, err := d.sendMsg(reqID, msg)
	if err != nil {
		return 0, err
	}

	return res.Battery, nil
}

func (d *RelayDevice) GetSpillover() (string, error) {
	reqID := makeID()
	msg, err := getVar(reqID, "spillover")
	if err != nil {
		return "", err
	}

	res, err := d.sendMsg(reqID, msg)
	if err != nil {
		return "", err
	}

	return res.Value, nil
}

func (d *RelayDevice) GetName() (string, error) {
	reqID := makeID()
	msg, err := deviceInfoMsg(reqID, "name", false)
	if err != nil {
		return "", err
	}

	res, err := d.sendMsg(reqID, msg)
	if err != nil {
		return "", err
	}

	return res.Name, nil
}

func (d *RelayDevice) GetLatLong() (string, error) {
	reqID := makeID()
	msg, err := deviceInfoMsg(reqID, "latlong", false)
	if err != nil {
		return "", err
	}

	res, err := d.sendMsg(reqID, msg)
	if err != nil {
		return "", err
	}

	return res.LatLong, nil
}

func (d *RelayDevice) SetChannel(name string) error {
	reqID := makeID()
	msg, err := setDeviceInfoMsg(reqID, "channel", name)
	if err != nil {
		return err
	}

	_, err = d.sendMsg(reqID, msg)
	if err != nil {
		return err
	}

	return nil
}

func (d *RelayDevice) GetIndoorLocation() (string, error) {
	reqID := makeID()
	msg, err := deviceInfoMsg(reqID, "indoor_location", false)
	if err != nil {
		return "", err
	}

	res, err := d.sendMsg(reqID, msg)
	if err != nil {
		return "", err
	}

	return res.IndoorLocation, nil
}

func (d *RelayDevice) Listen(phrases []string) (string, error) {
	reqID := makeID()
	msg, err := listenMsg(reqID, phrases)
	if err != nil {
		return "", err
	}

	res, err := d.sendMsg(reqID, msg)
	if err != nil {
		return "", err
	}

	return res.Text, nil
}

func (d *RelayDevice) Say(message string) error {
	reqID := makeID()
	msg, err := sayMsg(reqID, message)
	if err != nil {
		return err
	}

	_, err = d.sendMsg(reqID, msg)
	if err != nil {
		return err
	}

	return nil
}

func (d *RelayDevice) Vibrate() error {
	reqID := makeID()
	msg, err := vibrateMsg(reqID)
	if err != nil {
		return err
	}

	_, err = d.sendMsg(reqID, msg)
	if err != nil {
		return err
	}

	return nil
}

func (d *RelayDevice) LedStatic() error {
	reqID := makeID()
	msg, err := ledMsg(reqID, "static", LedArgs{
		Colors: LedColors{
			Ring: "FFFFFF",
		},
	})
	if err != nil {
		return err
	}

	_, err = d.sendMsg(reqID, msg)
	if err != nil {
		return err
	}

	return nil
}

func (d *RelayDevice) LedOff() error {
	reqID := makeID()
	msg, err := ledMsg(reqID, "off", LedArgs{})
	if err != nil {
		return err
	}

	_, err = d.sendMsg(reqID, msg)
	if err != nil {
		return err
	}

	return nil
}

// This function is not thread safe and should be called only once.
// Internal use only.
func (d *RelayDevice) terminate() error {
	defer close(d.stopC)

	reqID := makeID()
	msg, err := terminateMsg(reqID)
	if err != nil {
		return err
	}

	_, err = d.sendMsg(reqID, msg)

	d.isTerminatedMu.Lock()
	defer d.isTerminatedMu.Unlock()
	d.isTerminated = true

	return err
}

func (d *RelayDevice) Terminate() {
	d.terminateOnce.Do(func() { d.terminate() })
}

func (d *RelayDevice) IsTerminated() bool {
	d.isTerminatedMu.RLock()
	defer d.isTerminatedMu.RUnlock()

	return d.isTerminated
}

func (d *RelayDevice) sendMsg(reqID string, msg []byte) (*resMsg, error) {
	if d.IsTerminated() {
		return nil, errors.New("writing to a terminated relay device")
	}
	resC := make(chan resMsg)
	d.setMsgChan(reqID, resC)

	d.sendC <- []byte(msg)

	select {
	case res := <-resC:
		d.delMsgChan(reqID)
		close(resC)

		if res.Error != nil && res.Error != "" {
			return nil, errors.New(res.Error.(string))
		}

		return &res, nil
	case <-time.After(RES_TIMEOUT * time.Second):
		d.delMsgChan(reqID)
		close(resC)

		return nil, errors.New("request timed out")
	case <-d.stopC:
		d.delMsgChan(reqID)
		close(resC)

		return nil, errors.New("request timed out")
	}
}

func (d *RelayDevice) recvMsg() {
	for {
		select {
		case <-d.stopC:
			return
		case msg, ok := <-d.recvC:
			if !ok {
				return
			}

			m := resMsg{}
			err := json.Unmarshal(msg, &m)
			if err != nil {
				continue
			}

			c, err := d.getMsgChan(m.ReqID)
			if err != nil {
				continue
			}

			c <- m
		}

	}
}

func (d *RelayDevice) getMsgChan(resID string) (chan resMsg, error) {
	defer d.msgChanMu.RUnlock()
	d.msgChanMu.RLock()

	if c, ok := d.msgChanMap[resID]; ok {
		return c, nil
	}

	return nil, errors.New("channel not found")
}

func (d *RelayDevice) setMsgChan(resID string, resC chan resMsg) {
	defer d.msgChanMu.Unlock()
	d.msgChanMu.Lock()
	d.msgChanMap[resID] = resC
}

func (d *RelayDevice) delMsgChan(resID string) {
	defer d.msgChanMu.Unlock()
	d.msgChanMu.Lock()
	delete(d.msgChanMap, resID)
}

func getVar(id string, variable string) ([]byte, error) {
	msg := reqMsg{
		ReqID:   id,
		ReqType: "wf_api_get_var_request",
		Name:    variable,
	}

	return json.Marshal(&msg)
}

func deviceInfoMsg(id string, infoType string, refresh bool) ([]byte, error) {
	msg := reqMsg{
		ReqID:   id,
		ReqType: "wf_api_get_device_info_request",
		Query:   infoType,
		Refresh: refresh,
	}

	return json.Marshal(&msg)
}

func setDeviceInfoMsg(id string, field string, value string) ([]byte, error) {
	msg := reqSetDeviceInfo{
		ReqID:   id,
		ReqType: "wf_api_set_device_info_request",
		Field:   field,
		Value:   value,
	}

	return json.Marshal(&msg)
}

func sayMsg(id string, text string) ([]byte, error) {
	msg := reqSayMsg{
		ID:      id,
		ReqType: "wf_api_say_request",
		Text:    text,
		Lang:    "en-US",
	}

	return json.Marshal(&msg)
}

func listenMsg(id string, phrases []string) ([]byte, error) {
	msg := reqListenMsg{
		ReqID:      id,
		ReqType:    "wf_api_listen_request",
		Transcribe: true,
		Phrases:    phrases,
		Timeout:    RES_TIMEOUT,
		AltLang:    "en-US",
	}

	return json.Marshal(&msg)
}

func vibrateMsg(id string) ([]byte, error) {
	msg := reqVibrateMsg{
		ReqID:   id,
		ReqType: "wf_api_vibrate_request",
		Pattern: []int{100, 500, 500, 500, 500, 500},
	}

	return json.Marshal(&msg)
}

func ledMsg(id string, effect string, args LedArgs) ([]byte, error) {
	msg := reqVibrateMsg{
		ReqID:   id,
		ReqType: "wf_api_vibrate_request",
		Pattern: []int{100, 500, 500, 500, 500, 500},
	}

	return json.Marshal(&msg)
}

func terminateMsg(id string) ([]byte, error) {
	msg := reqMsg{
		ReqID:   id,
		ReqType: "wf_api_terminate_request",
	}

	return json.Marshal(&msg)
}

func makeID() string {
	rand.Seed(time.Now().UnixNano())
	var letters = []rune("abcdef1234567890")

	b := make([]rune, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

// resMsg is data structure for bidirectional communication with a relay device.
type resMsg struct {
	ReqID             string      `json:"_id"`
	ReqType           string      `json:"_type"`
	Error             interface{} `json:"error"`
	Button            string      `json:"button"`
	Taps              string      `json:"taps"`
	Source            string      `json:"source"`
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Event             string      `json:"event"`
	NotificationState string      `json:"notification_state"`
	IndoorLocation    string      `json:"indoor_location"`
	LatLong           string      `json:"latlong"`
	Text              string      `json:"text"`
	Battery           int64       `json:"battery"`
	Value             string      `json:"value"`
}

type reqMsg struct {
	ReqID   string `json:"_id"`
	ReqType string `json:"_type"`
	Query   string `json:"query,omitempty"`
	Refresh bool   `json:"refresh,omitempty"`
	Name    string `json:"name,omitempty" `
}

type reqSayMsg struct {
	ID      string `json:"_id"`
	ReqType string `json:"_type"`
	Text    string `json:"text"`
	Lang    string `json:"lang"`
}

type reqVibrateMsg struct {
	ReqID   string `json:"_id"`
	ReqType string `json:"_type"`
	Pattern []int  `json:"pattern"`
}

type reqLedMsg struct {
	ID      string  `json:"_id"`
	ReqType string  `json:"_type"`
	Effect  string  `json:"pattern"`
	Args    LedArgs `json:"args"`
}

type LedArgs struct {
	Colors LedColors `json:"colors"`
}

type LedColors struct {
	Ring string `json:"ring"`
}

// TODO: Use `reqMsg` as a wrapper for `_id`,`_type` and other common parts.
type reqListenMsg struct {
	ReqID      string   `json:"_id"`
	ReqType    string   `json:"_type"`
	Transcribe bool     `json:"transcribe"`
	Phrases    []string `json:"phrases"`
	Timeout    int      `json:"timeout"`
	AltLang    string   `json:"alt_lang"`
}

type reqSetDeviceInfo struct {
	ReqID   string `json:"_id"`
	ReqType string `json:"_type"`
	Field   string `json:"field"`
	Value   string `json:"value"`
}
