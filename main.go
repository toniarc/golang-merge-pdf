package main

import (
	"encoding/json"
	"fmt"
	"gerar_unico/pdf"
	"gerar_unico/s3"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/bradhe/stopwatch"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

var protocoloServiceBaseUrl = os.Getenv("PAE_ENV_PROTOCOLO_HOST") + "/pae-protocolo-producer-service"
var anexoServiceBaseUrl = os.Getenv("PAE_ENV_ANEXO_SERVICE_HOST") + "/pae-anexo-service"
var context = "/pae-gerar-documento-unico-service"
var tmp = "/tmp/"
var minioClient = s3.CreateClient()

func main() {
	router := httprouter.New()
	router.POST(context+"/documento-unico/:ano/:numero", GerarDocumentoUnico)
	router.GET(context+"/documento-unico/:tempFileId", Download)

	port := os.Getenv("PAE_ENV_GERAR_DOCUMENTO_SERVICE_LOCAL_SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	http.ListenAndServe(":"+port, router)
	fmt.Println("Server running on port " + port)
}

func Download(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	tempFileId := ps.ByName("tempFileId")

	file, err := s3.Donwload(tempFileId, minioClient)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/pdf")
	w.Write(file)
}

func GerarDocumentoUnico(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	ano, _ := strconv.Atoi(ps.ByName("ano"))
	numero, _ := strconv.Atoi(ps.ByName("numero"))
	var anexosIds []int

	if r.URL.Query().Get("anexosIds") != "" {
		strArray := strings.Split(r.URL.Query().Get("anexosIds"), ",")

		for _, str := range strArray {
			num, _ := strconv.Atoi(str)
			anexosIds = append(anexosIds, num)
		}
	}

	token := r.Header.Get("Authorization")

	protocolo := buscarProtocolo(ano, numero, token)

	if reflect.DeepEqual(protocolo, Protocolo{}) {
		sendBadRequestResponse(w, fmt.Sprintf("O protocolo %d/%d não existe", ano, numero))
	}

	if len(anexosIds) > 0 {

		for _, id := range anexosIds {
			if !contains(protocolo.Anexos, id) {
				sendBadRequestResponse(w, fmt.Sprintf("O anexo com o id %d não existe ou não pertence ao protocolo %d/%d", id, ano, numero))
			}
		}

	} else {

		for _, anexo := range protocolo.Anexos {
			if anexo.Confirmado {
				anexosIds = append(anexosIds, anexo.ID)
			}
		}

	}

	var files []string

	capaProcessoFileId := buscarCapaProcesso(ano, numero, anexosIds, token)
	files = append(files, capaProcessoFileId)

	for i, id := range anexosIds {
		fmt.Printf("[Protocolo %d/%d] baixando anexo %d de %d...", ano, numero, i+1, len(anexosIds))
		watch := stopwatch.Start()
		files = append(files, buscarAnexoPdf(id, token))
		watch.Stop()
		fmt.Printf("finalizado em: %v\n", watch.Milliseconds())
	}

	id, _ := uuid.NewRandom()
	resultFile := tmp + id.String() + ".pdf"

	pdf.MergePdf(files, resultFile)

	for _, file := range files {
		os.Remove(file)
	}

	tempFileId, _ := uuid.NewRandom()
	s3.Upload(tempFileId.String(), resultFile, minioClient)

	os.Remove(resultFile)

	w.Write([]byte(tempFileId.String()))
}

func buscarProtocolo(ano int, numero int, token string) Protocolo {

	fmt.Printf("Buscando protocolo %d/%d...", ano, numero)

	watch := stopwatch.Start()

	requestURL := protocoloServiceBaseUrl + fmt.Sprintf("/protocolos/%d/%d?trazerAnexos=true", ano, numero)

	req, _ := http.NewRequest(http.MethodGet, requestURL, nil)
	req.Header.Set("Authorization", token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	body, _ := ioutil.ReadAll(res.Body)

	var protocolo Protocolo
	json.Unmarshal(body, &protocolo)

	watch.Stop()
	fmt.Printf("Finalizado em: %v\n", watch.Milliseconds())

	return protocolo
}

func buscarCapaProcesso(ano int, numero int, anexosIds []int, token string) string {

	fmt.Printf("Buscando capa do processo %d/%d...", ano, numero)

	watch := stopwatch.Start()

	requestURL := protocoloServiceBaseUrl + fmt.Sprintf("/protocolos/%d/%d/capa-processo", ano, numero)

	req, _ := http.NewRequest(http.MethodGet, requestURL, nil)
	req.Header.Set("Authorization", token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	// Create blank file
	id, _ := uuid.NewRandom()
	fileName := tmp + id.String() + ".pdf"
	file, err := os.Create(fileName)

	io.Copy(file, res.Body)

	defer res.Body.Close()

	defer file.Close()

	watch.Stop()
	fmt.Printf("Finalizado em: %v\n", watch.Milliseconds())

	return fileName
}

func buscarAnexoPdf(anexoId int, token string) string {
	requestURL := anexoServiceBaseUrl + fmt.Sprintf("/anexos/%d/pdf", anexoId)

	req, _ := http.NewRequest(http.MethodGet, requestURL, nil)
	req.Header.Set("Authorization", token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	// Create blank file
	id, _ := uuid.NewRandom()
	fileName := tmp + id.String() + ".pdf"
	file, err := os.Create(fileName)

	io.Copy(file, res.Body)

	defer res.Body.Close()

	defer file.Close()

	return fileName
}

func sendBadRequestResponse(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["message"] = message
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Write(jsonResp)
}

func contains(anexos []Anexo, id int) bool {
	for _, anexo := range anexos {
		if anexo.ID == id {
			return true
		}
	}

	return false
}

type Protocolo struct {
	AnoProtocolo    int    `json:"anoProtocolo"`
	NumeroProtocolo int    `json:"numeroProtocolo"`
	DataProtocolo   string `json:"dataProtocolo"`
	Especie         struct {
		ID int `json:"id"`
	} `json:"especie"`
	Assunto struct {
		ID int `json:"id"`
	} `json:"assunto"`
	Municipio struct {
		CodigoIbge string `json:"codigoIbge"`
	} `json:"municipio"`
	OrigemDocumento      string `json:"origemDocumento"`
	Complemento          string `json:"complemento"`
	Prioridade           string `json:"prioridade"`
	DocumentoProtocolado struct {
		ID                int    `json:"id"`
		Ano               int    `json:"ano"`
		Numero            int    `json:"numero"`
		CadastroUsuarioID int    `json:"cadastroUsuarioId"`
		CadastroData      string `json:"cadastroData"`
	} `json:"documentoProtocolado"`
	OrgaoDestino struct {
		ID int `json:"id"`
	} `json:"orgaoDestino"`
	LocalizacaoDestino struct {
		ID int `json:"id"`
	} `json:"localizacaoDestino"`
	HashAnexos    string `json:"hashAnexos"`
	HashAlgoritmo string `json:"hashAlgoritmo"`
	OrgaoOrigem   struct {
		ID int `json:"id"`
	} `json:"orgaoOrigem"`
	LocalizacaoOrigem struct {
		ID int `json:"id"`
	} `json:"localizacaoOrigem"`
	Anexos                            []Anexo `json:"anexos"`
	Arquivado                         bool    `json:"arquivado"`
	PendenciaRetornoDistribuicao      bool    `json:"pendenciaRetornoDistribuicao"`
	PendenciaAssinatura               bool    `json:"pendenciaAssinatura"`
	PendenciaConfirmacao              bool    `json:"pendenciaConfirmacao"`
	PendenciaConfirmacaoTermoAnulacao bool    `json:"pendenciaConfirmacaoTermoAnulacao"`
	OrgaoAtual                        struct {
		ID int `json:"id"`
	} `json:"orgaoAtual"`
	LocalizacaoAtual struct {
		ID int `json:"id"`
	} `json:"localizacaoAtual"`
	TipoProtocolo          string `json:"tipoProtocolo"`
	CadastroUsuarioID      int    `json:"cadastroUsuarioId"`
	CadastroUsuarioNome    string `json:"cadastroUsuarioNome"`
	CadastroData           string `json:"cadastroData"`
	AtualizacaoUsuarioID   int    `json:"atualizacaoUsuarioId"`
	AtualizacaoUsuarioNome string `json:"atualizacaoUsuarioNome"`
	AtualizacaoData        string `json:"atualizacaoData"`
	Versao                 int    `json:"versao"`
}

type Anexo struct {
	ID                  int     `json:"id"`
	AnoProtocolo        int     `json:"anoProtocolo"`
	NumeroProtocolo     int     `json:"numeroProtocolo"`
	TipoDocumento       string  `json:"tipoDocumento"`
	TipoCriacao         string  `json:"tipoCriacao"`
	S3StorageID         string  `json:"s3StorageId"`
	NomeArquivo         string  `json:"nomeArquivo"`
	TamanhoArquivoMb    float64 `json:"tamanhoArquivoMb"`
	QuantidadePaginas   int     `json:"quantidadePaginas"`
	DocumentoInicial    bool    `json:"documentoInicial"`
	HashArquivo         string  `json:"hashArquivo"`
	HashAlgoritmo       string  `json:"hashAlgoritmo"`
	Versao              int     `json:"versao"`
	LocalizacaoCadastro struct {
		ID int `json:"id"`
	} `json:"localizacaoCadastro"`
	Assinaturas                 []Assinatura `json:"assinaturas"`
	UsuariosPendenciaAssinatura []any        `json:"usuariosPendenciaAssinatura"`
	AssinadoPorTodos            bool         `json:"assinadoPorTodos"`
	QuantidadeAssinaturas       int          `json:"quantidadeAssinaturas"`
	TipoAssinatura              string       `json:"tipoAssinatura"`
	Confirmado                  bool         `json:"confirmado"`
	Sequencial                  int          `json:"sequencial,omitempty"`
	ConfirmacaoUsuarioID        int          `json:"confirmacaoUsuarioId,omitempty"`
	ConfirmacaoData             string       `json:"confirmacaoData,omitempty"`
	CodigoValidacao             string       `json:"codigoValidacao,omitempty"`
	CadastroUsuarioID           int          `json:"cadastroUsuarioId"`
	CadastroUsuarioNome         string       `json:"cadastroUsuarioNome"`
	CadastroData                string       `json:"cadastroData"`
	AtualizacaoUsuarioID        int          `json:"atualizacaoUsuarioId"`
	AtualizacaoUsuarioNome      string       `json:"atualizacaoUsuarioNome"`
	AtualizacaoData             string       `json:"atualizacaoData"`
	ModeloEstruturaID           int          `json:"modeloEstruturaId,omitempty"`
	Especie                     struct {
		ID            int    `json:"id"`
		Nome          string `json:"nome"`
		GeraProtocolo bool   `json:"geraProtocolo"`
	} `json:"especie,omitempty"`
	TipoAssinaturaDigital string `json:"tipoAssinaturaDigital,omitempty"`
}

type Assinatura struct {
	ID                         int    `json:"id"`
	AnexoID                    int    `json:"anexoId"`
	UsuarioAssinanteID         int    `json:"usuarioAssinanteId"`
	UsuarioAssinanteNome       string `json:"usuarioAssinanteNome"`
	UsuarioAssinanteCpf        string `json:"usuarioAssinanteCpf"`
	UsuarioAssinanteOrgaoID    int    `json:"usuarioAssinanteOrgaoId"`
	UsuarioAssinanteOrgaoSigla string `json:"usuarioAssinanteOrgaoSigla"`
	TipoAssinatura             string `json:"tipoAssinatura"`
	DataRequisicao             string `json:"dataRequisicao"`
	DataAssinatura             string `json:"dataAssinatura"`
	UsuarioRequisitanteID      int    `json:"usuarioRequisitanteId"`
	Assinado                   bool   `json:"assinado"`
	DadosAssinatura            string `json:"dadosAssinatura"`
	HashAssinatura             string `json:"hashAssinatura"`
	VersaoAlgoritmoAssinatura  int    `json:"versaoAlgoritmoAssinatura"`
	CadastroUsuarioID          int    `json:"cadastroUsuarioId"`
	CadastroUsuarioNome        string `json:"cadastroUsuarioNome"`
	CadastroData               string `json:"cadastroData"`
	AtualizacaoUsuarioID       int    `json:"atualizacaoUsuarioId"`
	AtualizacaoUsuarioNome     string `json:"atualizacaoUsuarioNome"`
	AtualizacaoData            string `json:"atualizacaoData"`
}
