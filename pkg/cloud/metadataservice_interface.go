package cloud

// MetadataService represents Power VS metadata service.
type MetadataService interface {
	GetServiceInstanceId() string
	GetInstanceRegion() string
	GetNodeInstanceId() string
}
