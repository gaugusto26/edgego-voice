package capcuttts

import (
	"testing"
)

func TestCatalogResolution(t *testing.T) {
	cat := DefaultCatalog()

	tests := []struct {
		input            string
		expectedVoice    string
		expectedResource string
	}{
		{"wenrou", "DiT_pt_male_wenrou", "7576131428711255297"},
		{"empresario", "DiT_pt_male_shangren", "7576131970103643408"},
		{"luiz", "ICL_br_male_212M_LuizS", "7501618011829178896"},
		{"youya", "DiT_pt_female_youya", "7576144104547945729"},
		{"emotional", "en_female_emotional_moon_bigtts", "7114563483257016833"},
	}

	for _, tc := range tests {
		v, found := cat.ResolveVoice(tc.input)
		if !found {
			t.Errorf("Esperava encontrar a voz para input '%s'", tc.input)
			continue
		}
		if v.VoiceType != tc.expectedVoice {
			t.Errorf("Para '%s', esperava VoiceType '%s', obteve '%s'", tc.input, tc.expectedVoice, v.VoiceType)
		}
		if v.ResourceID != tc.expectedResource {
			t.Errorf("Para '%s', esperava ResourceID '%s', obteve '%s'", tc.input, tc.expectedResource, v.ResourceID)
		}
	}
}

func TestIsCapCutVoice(t *testing.T) {
	cat := DefaultCatalog()

	if !cat.IsCapCutVoice("wenrou") {
		t.Errorf("Esperava 'wenrou' ser reconhecida como CapCut")
	}
	if !cat.IsCapCutVoice("DiT_pt_male_wenrou") {
		t.Errorf("Esperava 'DiT_pt_male_wenrou' ser reconhecida como CapCut")
	}
	if cat.IsCapCutVoice("pt-BR-FranciscaNeural") {
		t.Errorf("Não esperava 'pt-BR-FranciscaNeural' ser reconhecida como CapCut")
	}
}

func TestRSASigning(t *testing.T) {
	ssml := "<speak>Teste</speak>"
	sign, err := makeTTSPayloadSign(ssml, "{}", "test_device", "359289")
	if err != nil {
		t.Fatalf("Erro ao assinar payload: %v", err)
	}
	if len(sign) < 50 {
		t.Errorf("Assinatura RSA muito curta: %s", sign)
	}
}

func TestSignHeader(t *testing.T) {
	header := makeSignHeader("https://editor-api-sg.capcutapi.com/lv/v1/common_task/new", "8.7.0", "12345678", "test_tdid")
	if len(header) != 32 {
		t.Errorf("Assinatura de cabeçalho deve ter 32 caracteres hex (MD5), obteve %d", len(header))
	}
}
