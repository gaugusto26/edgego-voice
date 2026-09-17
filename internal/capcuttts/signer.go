package capcuttts

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const (
	defaultBaseURL = "https://editor-api-sg.capcutapi.com"
	rsaPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAmTd34Lw4b7IuldSXh/zY
CMla+ITdGG5TeWz6ad+OySd4r+IrY45AoqrYUxhQ2dl+7z+i7r/5vEa8rr39BYfB
8AGMQLmZA8HmgpWBsqrn/V6daUALkKnkLb70Fn32CJigIuGXAYqxUdGuI340aC+0
v5Es3puJsHyzf01/AelE4Cdc6bZhQrASJLBh8R3BQToYClmDVSDUQk28o8sl/guA
Z4n303Vj+6Siv1HayPCdV6kpVVnMBAG4+umUbwGmn132N3fgpzLarFF3XyWmS1zh
D/J07iM/rP8GDO9IskHNHd2phrO0G6KzrcFAnTBHjVv+hCBEfzN/no3FNA9AuC36
mwIDAQAB
-----END PUBLIC KEY-----`
)

// DeviceConfig armazena os identificadores do cliente CapCut emulado.
type DeviceConfig struct {
	AID            string `json:"aid"`
	AppName        string `json:"app_name"`
	AppVR          string `json:"appvr"`
	VersionName    string `json:"version_name"`
	VersionCode    string `json:"version_code"`
	Channel        string `json:"channel"`
	DevicePlatform string `json:"device_platform"`
	DeviceType     string `json:"device_type"`
	DeviceBrand    string `json:"device_brand"`
	OSVersion      string `json:"os_version"`
	DeviceID       string `json:"device_id"`
	IID            string `json:"iid"`
	Region         string `json:"region"`
	Loc            string `json:"loc"`
	Lan            string `json:"lan"`
	PF             string `json:"pf"`
	TDID           string `json:"tdid"`
}

// DefaultDeviceConfig retorna a configuração de dispositivo padrão e estável.
func DefaultDeviceConfig() DeviceConfig {
	return DeviceConfig{
		AID:            "359289",
		AppName:        "CapCut",
		AppVR:          "8.7.0",
		VersionName:    "8.7.0",
		VersionCode:    "8.7.0",
		Channel:        "capcutpc_google",
		DevicePlatform: "mac",
		DeviceType:     "MacBookPro17,4",
		DeviceBrand:    "MacBookPro17,4",
		OSVersion:      "15.7.4",
		DeviceID:       "76471456455646328721",
		IID:            "76471456455646328721",
		Region:         "VN",
		Loc:            "VN",
		Lan:            "vi-VN",
		PF:             "3",
		TDID:           "76471456455646328721",
	}
}

// rsaEncryptPKCS1v15 cifra uma mensagem utilizando a chave pública RSA no formato PKCS#1 v1.5.
func rsaEncryptPKCS1v15(pubKeyPEM, message string) (string, error) {
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return "", fmt.Errorf("falha ao decodificar bloco PEM da chave pública")
	}

	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("falha ao parsear chave pública: %w", err)
	}

	pubKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("chave pública não é do tipo RSA")
	}

	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, []byte(message))
	if err != nil {
		return "", fmt.Errorf("erro durante encriptação RSA PKCS#1 v1.5: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// md5Hex calcula o hash MD5 e retorna em representação hexadecimal minúscula.
func md5Hex(data string) string {
	h := md5.Sum([]byte(data))
	return hex.EncodeToString(h[:])
}

// makeSignHeader gera a assinatura HTTP MD5 exigida pelo gateway da API CapCut.
func makeSignHeader(reqURL, appvr, deviceTime, tdid string) string {
	path := strings.Split(reqURL, "?")[0]
	pathEnd := ""
	if len(path) >= 7 {
		pathEnd = path[len(path)-7:]
	} else {
		pathEnd = path
	}
	signStr := fmt.Sprintf("9e2c|%s|3|%s|%s|%s|11ac", pathEnd, appvr, deviceTime, tdid)
	return md5Hex(signStr)
}

// makeTTSPayloadSign gera a assinatura RSA embutida no payload da tarefa TTS.
func makeTTSPayloadSign(ssml, extraInfo, deviceID, appID string) (string, error) {
	ssmlMD5 := md5Hex(ssml)
	signInput := fmt.Sprintf("appid:%s&did:%s&creditDisable:false&ssml:%s&extraInfo:%s",
		appID, deviceID, ssmlMD5, extraInfo)
	return rsaEncryptPKCS1v15(rsaPublicKeyPEM, signInput)
}

// makeTraceID gera um cabeçalho de rastreamento compatível com o padrão W3C traceparent.
func makeTraceID() string {
	traceSeed := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("00-%s-%s-01", traceSeed, traceSeed[:16])
}

// escapeXML escapa caracteres reservados em blocos SSML.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// buildCommonQuery monta os parâmetros de consulta padrão da API CapCut.
func buildCommonQuery(device DeviceConfig, babiParam string, includeRegion bool) url.Values {
	q := url.Values{}
	q.Set("app_name", device.AppName)
	q.Set("device_type", device.DeviceType)
	q.Set("os_version", device.OSVersion)
	q.Set("channel", device.Channel)
	q.Set("version_name", device.VersionName)
	q.Set("device_brand", device.DeviceBrand)
	q.Set("device_id", device.DeviceID)
	q.Set("iid", device.IID)
	q.Set("version_code", device.VersionCode)
	q.Set("device_platform", device.DevicePlatform)
	q.Set("aid", device.AID)
	if includeRegion {
		q.Set("region", device.Region)
	}
	if babiParam != "" {
		q.Set("babi_param", babiParam)
	}
	return q
}

// buildBaseHeaders configura os cabeçalhos HTTP necessários para autenticação.
func buildBaseHeaders(device DeviceConfig, bodyText, now, signHeader string, appid bool) http.Header {
	h := make(http.Header)
	h.Set("content-type", "application/json")
	h.Set("appvr", device.AppVR)
	h.Set("ch", device.Channel)
	h.Set("device-time", now)
	h.Set("lan", device.Lan)
	h.Set("loc", device.Loc)
	h.Set("pf", device.PF)
	h.Set("sign-ver", "1")
	h.Set("tdid", device.TDID)
	h.Set("x-ss-stub", md5Hex(bodyText))
	h.Set("x-ss-dp", device.AID)
	h.Set("x-khronos", now)
	h.Set("x-tt-trace-id", makeTraceID())
	h.Set("user-agent", "Cronet/TTNetVersion:1d7cc3b1 2025-07-16 QuicVersion:52c2b40d 2025-04-03")
	h.Set("store-country-code", strings.ToLower(device.Loc))
	h.Set("store-country-code-src", "did")
	h.Set("is-dispatch-us-ttp", "0")
	h.Set("is-app-region-us-ttp", "0")
	h.Set("sign", signHeader)

	if appid {
		h.Set("app-sdk-version", device.AppVR)
		h.Set("appid", device.AID)
	}

	return h
}

// StripID3 remove o cabeçalho ID3v2 de arquivos MP3, retornando apenas os frames de áudio puros.
func StripID3(data []byte) []byte {
	if len(data) >= 10 && data[0] == 'I' && data[1] == 'D' && data[2] == '3' {
		size := int(data[6]&0x7F)<<21 | int(data[7]&0x7F)<<14 | int(data[8]&0x7F)<<7 | int(data[9]&0x7F)
		tagLen := 10 + size
		if data[5]&0x10 != 0 {
			tagLen += 10
		}
		if len(data) >= tagLen {
			return data[tagLen:]
		}
	}
	return data
}
