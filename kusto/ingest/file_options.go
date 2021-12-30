package ingest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-kusto-go/kusto/data/errors"
	"github.com/Azure/azure-kusto-go/kusto/ingest/internal/properties"
)

type OptionScope uint

const (
	IngestFromFile OptionScope = 1 << iota
	IngestFromReader
	IngestFromBlob
	QueuedIngest
	StreamingIngest
)

var scopesMap = map[OptionScope]string{
	IngestFromFile:   "IngestFromFile",
	IngestFromReader: "IngestFromReader",
	IngestFromBlob:   "IngestFromBlob",
	QueuedIngest:     "QueuedIngest",
	StreamingIngest:  "StreamingIngest",
}

// FileOption is an optional argument to FromFile().
type FileOption interface {
	fmt.Stringer

	OptionScopes() OptionScope

	Run(p *properties.All, scopes OptionScope) error
}

type option struct {
	run    func(p *properties.All) error
	scopes OptionScope
	name   string
}

func (o option) String() string {
	return o.name
}

func (o option) OptionScopes() OptionScope {
	return o.scopes
}

func (o option) Run(p *properties.All, scopes OptionScope) error {

	for scope, scopeName := range scopesMap {
		if ((scopes & scope) != 0) && ((o.scopes & scope) == 0) {
			errType := errors.OpFileIngest
			if (scopes & StreamingIngest) == 1 {
				errType = errors.OpIngestStream
			}
			return errors.ES(errType, errors.KClientArgs, "Option %v is not allowed in scope %s", o, scopeName)
		}
	}

	return o.run(p)
}

// FlushImmediately tells Kusto to flush on write.
func FlushImmediately() FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Ingestion.FlushImmediately = true
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | QueuedIngest,
		name:   "FlushImmediately",
	}
}

// DataFormat indicates what type of encoding format was used for source data.
// Not all options can be used in every method.
type DataFormat = properties.DataFormat

// note: any change here needs to be kept up to date with the properties version.
// I'm not a fan of having two copies, but I don't think it is worth moving to its own package
// to allow properties and ingest to both import without a cycle.
//goland:noinspection GoUnusedConst - Part of the API
const (
	// DFUnknown indicates the EncodingType is not set.
	DFUnknown DataFormat = properties.DFUnknown
	// AVRO indicates the source is encoded in Apache Avro format.
	AVRO DataFormat = properties.AVRO
	// ApacheAVRO indicates the source is encoded in Apache avro2json format.
	ApacheAVRO DataFormat = properties.ApacheAVRO
	// CSV indicates the source is encoded in comma seperated values.
	CSV DataFormat = properties.CSV
	// JSON indicates the source is encoded as one or more lines, each containing a record in Javascript Object Notation.
	JSON DataFormat = properties.JSON
	// MultiJSON indicates the source is encoded in JSON-Array of individual records in Javascript Object Notation. Optionally,
	//multiple documents can be concatenated.
	MultiJSON DataFormat = properties.MultiJSON
	// ORC indicates the source is encoded in Apache Optimized Row Columnar format.
	ORC DataFormat = properties.ORC
	// Parquet indicates the source is encoded in Apache Parquet format.
	Parquet DataFormat = properties.Parquet
	// PSV is pipe "|" separated values.
	PSV DataFormat = properties.PSV
	// Raw is a text file that has only a single string value.
	Raw DataFormat = properties.Raw
	// SCSV is a file containing semicolon ";" separated values.
	SCSV DataFormat = properties.SCSV
	// SOHSV is a file containing SOH-separated values(ASCII codepoint 1).
	SOHSV DataFormat = properties.SOHSV
	// SStream indicates the source is encoded as a Microsoft Cosmos Structured Streams format
	SStream DataFormat = properties.SStream
	// TSV is a file containing tab seperated values ("\t").
	TSV DataFormat = properties.TSV
	// TSVE is a file containing escaped-tab seperated values ("\t").
	TSVE DataFormat = properties.TSVE
	// TXT is a text file with lines ending with "\n".
	TXT DataFormat = properties.TXT
	// W3CLogFile indicates the source is encoded using W3C Extended Log File format
	W3CLogFile DataFormat = properties.W3CLogFile
	// SingleJSON indicates the source is a single JSON value -- newlines are regular whitespace.
	SingleJSON DataFormat = properties.SingleJSON
)

// IngestionMapping provides runtime mapping of the data being imported to the fields in the table.
// "ref" will be JSON encoded, so it can be any type that can be JSON marshalled. If you pass a string
// or []byte, it will be interpreted as already being JSON encoded.
// mappingKind can only be: CSV, JSON, AVRO, Parquet or ORC.
func IngestionMapping(mapping interface{}, mappingKind DataFormat) FileOption {
	return option{
		run: func(p *properties.All) error {
			if !mappingKind.IsValidMappingKind() {
				return errors.ES(
					errors.OpUnknown,
					errors.KClientArgs,
					"IngestionMapping() option does not support EncodingType %v", mappingKind,
				).SetNoRetry()
			}

			var j string
			switch v := mapping.(type) {
			case string:
				j = v
			case []byte:
				j = string(v)
			default:
				b, err := json.Marshal(mapping)
				if err != nil {
					return errors.ES(
						errors.OpUnknown,
						errors.KClientArgs,
						"IngestMapping option was passed to an Ingest.Ingestion call that was not a string, []byte or could be JSON encoded: %s", err,
					).SetNoRetry()
				}
				j = string(b)
			}

			p.Ingestion.Additional.IngestionMapping = j
			p.Ingestion.Additional.IngestionMappingType = mappingKind

			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | QueuedIngest,
		name:   "IngestionMapping",
	}
}

// IngestionMappingRef provides the name of a pre-created mapping for the data being imported to the fields in the table.
// mappingKind can only be: CSV, JSON, AVRO, Parquet or ORC.
// For more details, see: https://docs.microsoft.com/en-us/azure/kusto/management/create-ingestion-mapping-command
func IngestionMappingRef(refName string, mappingKind DataFormat) FileOption {
	return option{
		run: func(p *properties.All) error {
			if !mappingKind.IsValidMappingKind() {
				return errors.ES(errors.OpUnknown, errors.KClientArgs, "IngestionMappingRef() option does not support EncodingType %v", mappingKind).SetNoRetry()
			}
			p.Ingestion.Additional.IngestionMappingRef = refName
			p.Ingestion.Additional.IngestionMappingType = mappingKind
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | QueuedIngest | StreamingIngest,
		name:   "IngestionMappingRef",
	}
}

// DeleteSource deletes the source file from when it has been uploaded to Kusto.
func DeleteSource() FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Source.DeleteLocalSource = true
			return nil
		},
		scopes: IngestFromFile | QueuedIngest | StreamingIngest,
		name:   "DeleteSource",
	}
}

// IgnoreSizeLimit ignores the size limit for data ingestion.
func IgnoreSizeLimit() FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Ingestion.IgnoreSizeLimit = true
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | QueuedIngest,
		name:   "IgnoreSizeLimit",
	}
}

// Tags are tags to be associated with the ingested ata.
func Tags(tags []string) FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Ingestion.Additional.Tags = tags
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | QueuedIngest,
		name:   "Tags",
	}
}

// IfNotExists provides a string value that, if specified, prevents ingestion from succeeding if the table already
// has data tagged with an ingest-by: tag with the same value. This ensures idempotent data ingestion.
// For more information see: https://docs.microsoft.com/en-us/azure/kusto/management/extents-overview#ingest-by-extent-tags
func IfNotExists(ingestByTag string) FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Ingestion.Additional.IngestIfNotExists = ingestByTag
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | QueuedIngest,
		name:   "IfNotExists",
	}
}

// ReportResultToTable option requests that the ingestion status will be tracked in an Azure table.
// Note using Table status reporting is not recommended for high capacity ingestions, as it could slow down the ingestion.
// In such cases, it's recommended to enable it temporarily for debugging failed ingestions.
func ReportResultToTable() FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Ingestion.ReportLevel = properties.FailureAndSuccess
			p.Ingestion.ReportMethod = properties.ReportStatusToTable
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | QueuedIngest | StreamingIngest,
		name:   "ReportResultToTable",
	}
}

// SetCreationTime option allows the user to override the data creation time the retention policies are considered against
// If not set the data creation time is considered to be the time of ingestion
func SetCreationTime(t time.Time) FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Ingestion.Additional.CreationTime = t
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | QueuedIngest,
		name:   "SetCreationTime",
	}
}

// ValidationOption is an an option for validating the ingestion input data.
// These are defined as constants within this package.
type ValidationOption int8

//goland:noinspection GoUnusedConst - Part of the API
const (
	// VOUnknown indicates that a ValidationOption was not set.
	VOUnknown ValidationOption = 0
	// SameNumberOfFields indicates that all records ingested must have the same number of fields.
	SameNumberOfFields ValidationOption = 1
	// IgnoreNonDoubleQuotedFields indicates that fields that do not have double quotes should be ignored.
	IgnoreNonDoubleQuotedFields ValidationOption = 2
)

// ValidationImplication is a setting used to indicate what to do when a Validation Policy is violated.
// These are defined as constants within this package.
type ValidationImplication int8

//goland:noinspection GoUnusedConst - Part of the API
const (
	// FailIngestion indicates that any violation of the ValidationPolicy will cause the entire ingestion to fail.
	FailIngestion ValidationImplication = 0
	// IgnoreFailures indicates that failure of the ValidationPolicy will be ignored.
	IgnoreFailures ValidationImplication = 1
)

// ValPolicy sets a policy for validating data as it is sent for ingestion.
// For more information, see: https://docs.microsoft.com/en-us/azure/kusto/management/data-ingestion/
type ValPolicy struct {
	// Options provides an option that will flag data that does not validate.
	Options ValidationOption `json:"ValidationOptions"`
	// Implications sets what to do when a policy option is violated.
	Implications ValidationImplication `json:"ValidationImplications"`
}

// ValidationPolicy uses a ValPolicy to set our ingestion data validation policy. If not set, no validation policy
// is used.
// For more information, see: https://docs.microsoft.com/en-us/azure/kusto/management/data-ingestion/
func ValidationPolicy(policy ValPolicy) FileOption {
	return option{
		run: func(p *properties.All) error {
			b, err := json.Marshal(policy)
			if err != nil {
				return errors.ES(errors.OpUnknown, errors.KInternal, "bug: the ValPolicy provided would not JSON encode").SetNoRetry()
			}

			// You might be asking, what if we are just using blobstore? Well, then this option doesn't matter :)
			p.Ingestion.Additional.ValidationPolicy = string(b)
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | QueuedIngest,
		name:   "ValidationPolicy",
	}
}

// FileFormat can be used to indicate what type of encoding is supported for the file. This is only needed if
// the file extension is not present. A file like: "input.json.gz" or "input.json" does not need this option, while
// "input" would.
func FileFormat(et DataFormat) FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Ingestion.Additional.Format = et
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | QueuedIngest | StreamingIngest,
		name:   "FileFormat",
	}
}

// ClientRequestId is an identifier for the ingestion, that can later be queried.
func ClientRequestId(clientRequestId string) FileOption {
	return option{
		run: func(p *properties.All) error {
			p.Streaming.ClientRequestId = clientRequestId
			return nil
		},
		scopes: IngestFromFile | IngestFromReader | IngestFromBlob | StreamingIngest,
		name:   "ClientRequestId",
	}
}