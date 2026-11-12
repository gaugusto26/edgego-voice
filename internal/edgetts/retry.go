package edgetts

import "time"

// O Edge TTS fecha a conexao de vez em quando (reset do peer). Uma unica
// tentativa transformaria isso em audio ausente; ate tres tentativas com espera
// curta mantem a entrega sem repetir audio ja confirmado.
const (
	synthesisAttempts = 3
	synthesisBackoff  = 800 * time.Millisecond
)
