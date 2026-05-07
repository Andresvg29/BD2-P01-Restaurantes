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

// }agrega o actualiza un plato en el indice 'platos'
func IndexPlato(plato models.MenuItem) error {
	// Convertimos el modelo a JSON
	data, err := json.Marshal(plato)
	if err != nil {
		return fmt.Errorf("error serializando plato: %s", err)
	}

	// eviamos el documento a ES usando el ID del plato como identificador del documento
	res, err := ESClient.Index(
		"platos",
		bytes.NewReader(data),
		ESClient.Index.WithDocumentID(plato.ID),
		ESClient.Index.WithRefresh("true"), // cambio visible inmediatamente
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

// elimina un plato del indice cuando se borra de la BD
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

// permite limpiar el indice y volver a cargar todo desde la base de datos
func ReindexAll(platos []models.MenuItem) error {
	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{ //se usa para eficiencia si hay muchois datos
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
