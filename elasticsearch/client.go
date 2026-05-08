package elasticsearch

import (
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

var ESClient *elasticsearch.Client

// inicializa la conexión y prepara el indice
func InitClient(url string) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{url},
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creando el cliente de Elasticsearch: %s", err)
	}

	// Verificar conexion
	res, err := client.Info()
	if err != nil {
		log.Fatalf("Error conectando con Elasticsearch: %s", err)
	}
	defer res.Body.Close()

	ESClient = client
	log.Println("Conexión exitosa con Elasticsearch")

	// crear el indice 'platos' con su mapping inicial
	createPlatosIndex(client)
	return client, nil
}

func createPlatosIndex(client *elasticsearch.Client) {
	indexName := "platos"

	//definición del Mapping campos buscables y sus tipos
	//'text' para busqueda difusa y 'keyword' para filtros exactos
	mapping := `
	{
	  "settings": {
	    "number_of_shards": 1,
	    "number_of_replicas": 1
	  },
	  "mappings": {
	    "properties": {
	      "id":              { "type": "keyword" },
	      "id_restaurante":  { "type": "keyword" },
	      "nombre":          { "type": "text", "analyzer": "spanish" },
	      "descripcion":     { "type": "text", "analyzer": "spanish" },
	      "precio":          { "type": "double" },
	      "esta_disponible": { "type": "boolean" },
	      "categoria":       { "type": "keyword" }
	    }
	  }
	}`

	//verificar si el indice ya existe
	res, err := client.Indices.Exists([]string{indexName})
	if err != nil {
		log.Printf("Error verificando existencia del índice: %s", err)
		return
	}

	if res.StatusCode == 404 {
		//si no existe, lo creamos con el mapping
		res, err = client.Indices.Create(
			indexName,
			client.Indices.Create.WithBody(strings.NewReader(mapping)),
		)
		if err != nil {
			log.Printf("Error al crear el índice de platos: %s", err)
			return
		}
		defer res.Body.Close()
		log.Println("Índice 'platos' creado exitosamente con Mapping en español")
	} else {
		log.Println("El índice 'platos' ya existe, saltando creación")
	}
}
