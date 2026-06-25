package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"restaurante/models"

	"github.com/elastic/go-elasticsearch/v8/esutil"
)

// adds or updates a dish in the 'platos' index
func IndexPlato(plato models.MenuItem) error {
	// Convert the model to JSON
	data, err := json.Marshal(plato)
	if err != nil {
		return fmt.Errorf("error serializando plato: %s", err)
	}

	// send the document to ES using the dish ID as the document identifier
	res, err := ESClient.Index(
		"platos",
		bytes.NewReader(data),
		ESClient.Index.WithDocumentID(plato.ID),
		ESClient.Index.WithRefresh("true"), // immediately visible
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error al indexar: %s", res.String())
	}

	log.Printf("Plato [%s] indexado correctamente en ES", plato.Nombre)
	return nil
}

// removes a dish from the index when it is deleted from the DB
func DeletePlato(platoID string) error {
	res, err := ESClient.Delete(
		"platos",
		platoID,
		ESClient.Delete.WithRefresh("true"),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("error al eliminar de ES: %s", res.String())
	}

	log.Printf("Plato ID [%s] eliminado de ES", platoID)
	return nil
}

// allows clearing the index and reloading everything from the database
func ReindexAll(platos []models.MenuItem) error {
	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{ // used for efficiency when there is a lot of data
		Index:  "platos",
		Client: ESClient,
	})
	if err != nil {
		return err
	}

	for _, plato := range platos {
		data, _ := json.Marshal(plato)
		err := bi.Add(
			context.Background(),
			esutil.BulkIndexerItem{
				Action:     "index",
				DocumentID: plato.ID,
				Body:       bytes.NewReader(data),
			},
		)
		if err != nil {
			log.Printf("Error añadiendo al bulk: %s", err)
		}
	}

	return bi.Close(context.Background())
}
