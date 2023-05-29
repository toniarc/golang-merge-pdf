package s3

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bradhe/stopwatch"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func CreateClient() minio.Client {
	endpoint := os.Getenv("PAE_ENV_NUTANIX_ENDPOINT_URL")[7:]
	accessKeyID := os.Getenv("PAE_ENV_NUTANIX_ACCESS_KEY")
	secretAccessKey := os.Getenv("PAE_ENV_NUTANIX_SECRET_KEY")
	useSSL := false

	fmt.Printf("Criando client do cluster %s\n", endpoint)

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("%#v\n", minioClient) // minioClient is now set up
	return *minioClient
}

func Upload(id string, filePath string, minioClient minio.Client) {

	fmt.Printf("Enviando para o nutanix ...")
	watch := stopwatch.Start()

	ctx := context.Background()
	bucketName := os.Getenv("PAE_ENV_BUCKET_TEMPORARIOS")

	objectName := id
	contentType := "application/pdf"

	// Upload the zip file with FPutObject
	info, err := minioClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Successfully uploaded %s of size %d\n", objectName, info.Size)

	watch.Stop()
	fmt.Printf("concluído em: %v\n", watch.Milliseconds())

}

func Donwload(id string, minioClient minio.Client) (file []byte, err error) {

	fmt.Printf("Baixando o arquivo %s do nutanix ...", id)
	watch := stopwatch.Start()

	ctx := context.Background()
	bucketName := os.Getenv("PAE_ENV_BUCKET_TEMPORARIOS")

	//objectName := id
	//contentType := "application/pdf"

	objectResult, e := minioClient.GetObject(ctx, bucketName, id, minio.GetObjectOptions{})
	if e != nil {
		fmt.Println(err)
		return []byte{}, err
	}
	defer objectResult.Close()

	watch.Stop()
	fmt.Printf("concluído em: %v\n", watch.Milliseconds())

	info, _ := objectResult.Stat()

	byteArray := make([]byte, info.Size)

	objectResult.Read(byteArray)

	return byteArray, err
}
