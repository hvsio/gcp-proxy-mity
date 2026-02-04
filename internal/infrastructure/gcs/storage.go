package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"

	"gcp-proxy-mity/internal/service"
	gcsclient "gcp-proxy-mity/pkg/storage/gcs"

	cloudstorage "cloud.google.com/go/storage"
)

// Storage implements service.Storage using Google Cloud Storage.
type Storage struct {
	client *gcsclient.Client
}

// NewStorage creates a new GCS storage implementation.
func NewStorage(client *gcsclient.Client) *Storage {
	return &Storage{
		client: client,
	}
}

// WriteFiles implements service.Storage using a worker pool for concurrent uploads.
func (s *Storage) WriteFiles(ctx context.Context, requests []service.WriteRequest) (*service.WriteResponse, error) {
	if len(requests) == 0 {
		return &service.WriteResponse{
			FilesWritten: make([]service.FileMetadata, 0),
			Errors:       make([]service.WriteError, 0),
		}, nil
	}

	// For single file uploads, use direct processing to avoid overhead
	if len(requests) == 1 {
		return s.writeSingleFile(ctx, requests[0])
	}

	// Use worker pool for bulk uploads
	return s.writeFilesWithWorkerPool(ctx, requests)
}

// writeSingleFile handles single file upload without worker pool overhead
func (s *Storage) writeSingleFile(ctx context.Context, req service.WriteRequest) (*service.WriteResponse, error) {
	response := &service.WriteResponse{
		FilesWritten: make([]service.FileMetadata, 0),
		Errors:       make([]service.WriteError, 0),
	}

	bucket := s.client.GetBucket()
	metadata, err := s.uploadFile(ctx, bucket, req)
	if err != nil {
		response.Errors = append(response.Errors, service.WriteError{
			FilePath: req.Path,
			Error:    err.Error(),
		})
	} else {
		response.FilesWritten = append(response.FilesWritten, *metadata)
	}

	return response, nil
}

// writeFilesWithWorkerPool handles bulk uploads using a worker pool pattern
func (s *Storage) writeFilesWithWorkerPool(ctx context.Context, requests []service.WriteRequest) (*service.WriteResponse, error) {
	const maxWorkers = 10
	numWorkers := maxWorkers
	if len(requests) < numWorkers {
		numWorkers = len(requests)
	}

	// Create channels for work distribution and result collection
	jobChan := make(chan service.WriteRequest, len(requests))
	resultChan := make(chan uploadResult, len(requests))

	// Create a context that can be cancelled if any critical error occurs
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go s.uploadWorker(workerCtx, jobChan, resultChan)
	}

	// Send jobs to workers
	go func() {
		defer close(jobChan)
		for _, req := range requests {
			select {
			case jobChan <- req:
			case <-workerCtx.Done():
				return // Context cancelled, stop sending jobs
			}
		}
	}()

	// Collect results
	response := &service.WriteResponse{
		FilesWritten: make([]service.FileMetadata, 0),
		Errors:       make([]service.WriteError, 0),
	}

	for i := 0; i < len(requests); i++ {
		select {
		case result := <-resultChan:
			if result.Error != nil {
				response.Errors = append(response.Errors, service.WriteError{
					FilePath: result.FilePath,
					Error:    result.Error.Error(),
				})
			} else {
				response.FilesWritten = append(response.FilesWritten, *result.Metadata)
			}
		case <-ctx.Done():
			// Context cancelled, return partial results
			return response, ctx.Err()
		}
	}

	return response, nil
}

// uploadResult represents the result of a file upload operation
type uploadResult struct {
	FilePath string
	Metadata *service.FileMetadata
	Error    error
}

// uploadWorker is a worker goroutine that processes upload requests
func (s *Storage) uploadWorker(ctx context.Context, jobs <-chan service.WriteRequest, results chan<- uploadResult) {
	bucket := s.client.GetBucket()
	
	for {
		select {
		case req, ok := <-jobs:
			if !ok {
				return // Channel closed, worker should exit
			}
			
			// Process the upload request
			metadata, err := s.uploadFile(ctx, bucket, req)
			
			// Send result back
			select {
			case results <- uploadResult{
				FilePath: req.Path,
				Metadata: metadata,
				Error:    err,
			}:
			case <-ctx.Done():
				return // Context cancelled, worker should exit
			}
			
		case <-ctx.Done():
			return // Context cancelled, worker should exit
		}
	}
}

// uploadFile performs the actual file upload operation
func (s *Storage) uploadFile(ctx context.Context, bucket *cloudstorage.BucketHandle, req service.WriteRequest) (*service.FileMetadata, error) {
	obj := bucket.Object(req.Path)
	writer := obj.NewWriter(ctx)

	// Set content type
	if req.ContentType != "" {
		writer.ContentType = req.ContentType
	} else {
		writer.ContentType = mime.TypeByExtension(getExtension(req.Path))
	}

	// Upload the file content
	written, err := io.Copy(writer, req.Content)
	if err != nil {
		writer.Close() // Close writer on error
		return nil, fmt.Errorf("failed to upload file content: %w", err)
	}

	// Close the writer to finalize the upload
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize upload: %w", err)
	}

	// Get file attributes for metadata
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get file attributes: %w", err)
	}

	return &service.FileMetadata{
		Name:        req.Path,
		ContentType: attrs.ContentType,
		Size:        written,
	}, nil
}

// ReadFiles implements service.Storage using a worker pool for concurrent reads.
func (s *Storage) ReadFiles(ctx context.Context, filePaths []string) (*service.ReadResponse, error) {
	if len(filePaths) == 0 {
		return &service.ReadResponse{
			Files:  make([]service.FileData, 0),
			Errors: make([]service.ReadError, 0),
		}, nil
	}

	// For single file reads, use direct processing to avoid overhead
	if len(filePaths) == 1 {
		return s.readSingleFileResponse(ctx, filePaths[0])
	}

	// Use worker pool for bulk reads
	return s.readFilesWithWorkerPool(ctx, filePaths)
}

// readSingleFileResponse handles single file read without worker pool overhead
func (s *Storage) readSingleFileResponse(ctx context.Context, filePath string) (*service.ReadResponse, error) {
	response := &service.ReadResponse{
		Files:  make([]service.FileData, 0),
		Errors: make([]service.ReadError, 0),
	}

	bucket := s.client.GetBucket()
	fileData, err := s.readSingleFile(ctx, bucket, filePath)
	if err != nil {
		response.Errors = append(response.Errors, service.ReadError{
			FilePath: filePath,
			Error:    err.Error(),
		})
	} else {
		response.Files = append(response.Files, *fileData)
	}

	return response, nil
}

// readFilesWithWorkerPool handles bulk reads using a worker pool pattern
func (s *Storage) readFilesWithWorkerPool(ctx context.Context, filePaths []string) (*service.ReadResponse, error) {
	const maxWorkers = 10
	numWorkers := maxWorkers
	if len(filePaths) < numWorkers {
		numWorkers = len(filePaths)
	}

	// Create channels for work distribution and result collection
	jobChan := make(chan string, len(filePaths))
	resultChan := make(chan readResult, len(filePaths))

	// Create a context that can be cancelled if any critical error occurs
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go s.readWorker(workerCtx, jobChan, resultChan)
	}

	// Send jobs to workers
	go func() {
		defer close(jobChan)
		for _, filePath := range filePaths {
			select {
			case jobChan <- filePath:
			case <-workerCtx.Done():
				return // Context cancelled, stop sending jobs
			}
		}
	}()

	// Collect results
	response := &service.ReadResponse{
		Files:  make([]service.FileData, 0),
		Errors: make([]service.ReadError, 0),
	}

	for i := 0; i < len(filePaths); i++ {
		select {
		case result := <-resultChan:
			if result.Error != nil {
				response.Errors = append(response.Errors, service.ReadError{
					FilePath: result.FilePath,
					Error:    result.Error.Error(),
				})
			} else {
				response.Files = append(response.Files, *result.FileData)
			}
		case <-ctx.Done():
			// Context cancelled, return partial results
			return response, ctx.Err()
		}
	}

	return response, nil
}

// readResult represents the result of a file read operation
type readResult struct {
	FilePath string
	FileData *service.FileData
	Error    error
}

// readWorker is a worker goroutine that processes read requests
func (s *Storage) readWorker(ctx context.Context, jobs <-chan string, results chan<- readResult) {
	bucket := s.client.GetBucket()
	
	for {
		select {
		case filePath, ok := <-jobs:
			if !ok {
				return // Channel closed, worker should exit
			}
			
			// Process the read request
			fileData, err := s.readSingleFile(ctx, bucket, filePath)
			
			// Send result back
			select {
			case results <- readResult{
				FilePath: filePath,
				FileData: fileData,
				Error:    err,
			}:
			case <-ctx.Done():
				return // Context cancelled, worker should exit
			}
			
		case <-ctx.Done():
			return // Context cancelled, worker should exit
		}
	}
}

// ReadFile implements service.Storage.
func (s *Storage) ReadFile(ctx context.Context, filePath string) (*service.FileData, error) {
	bucket := s.client.GetBucket()
	return s.readSingleFile(ctx, bucket, filePath)
}

func (s *Storage) readSingleFile(ctx context.Context, bucket *cloudstorage.BucketHandle, filePath string) (*service.FileData, error) {
	obj := bucket.Object(filePath)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object attributes: %w", err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	return &service.FileData{
		Metadata: service.FileMetadata{
			Name:        filePath,
			ContentType: attrs.ContentType,
			Size:        attrs.Size,
		},
		Content: content,
	}, nil
}

// DeleteFile implements service.Storage.
func (s *Storage) DeleteFile(ctx context.Context, filePath string) error {
	bucket := s.client.GetBucket()
	obj := bucket.Object(filePath)
	if err := obj.Delete(ctx); err != nil {
		if errors.Is(err, cloudstorage.ErrObjectNotExist) {
			return service.ErrNotFound
		}
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// DeleteFiles implements service.Storage using a worker pool for concurrent deletes.
func (s *Storage) DeleteFiles(ctx context.Context, filePaths []string) (*service.DeleteResponse, error) {
	if len(filePaths) == 0 {
		return &service.DeleteResponse{
			Deleted: make([]string, 0),
			Errors:  make([]service.DeleteError, 0),
		}, nil
	}

	// For single file deletes, use direct processing to avoid overhead
	if len(filePaths) == 1 {
		return s.deleteSingleFileResponse(ctx, filePaths[0])
	}

	// Use worker pool for bulk deletes
	return s.deleteFilesWithWorkerPool(ctx, filePaths)
}

// deleteSingleFileResponse handles single file delete without worker pool overhead
func (s *Storage) deleteSingleFileResponse(ctx context.Context, filePath string) (*service.DeleteResponse, error) {
	response := &service.DeleteResponse{
		Deleted: make([]string, 0),
		Errors:  make([]service.DeleteError, 0),
	}

	bucket := s.client.GetBucket()
	err := s.deleteSingleFile(ctx, bucket, filePath)
	if err != nil {
		errMsg := err.Error()
		if errors.Is(err, cloudstorage.ErrObjectNotExist) {
			errMsg = service.ErrNotFound.Error()
		}
		response.Errors = append(response.Errors, service.DeleteError{
			FilePath: filePath,
			Error:    errMsg,
		})
	} else {
		response.Deleted = append(response.Deleted, filePath)
	}

	return response, nil
}

// deleteFilesWithWorkerPool handles bulk deletes using a worker pool pattern
func (s *Storage) deleteFilesWithWorkerPool(ctx context.Context, filePaths []string) (*service.DeleteResponse, error) {
	const maxWorkers = 10
	numWorkers := maxWorkers
	if len(filePaths) < numWorkers {
		numWorkers = len(filePaths)
	}

	// Create channels for work distribution and result collection
	jobChan := make(chan string, len(filePaths))
	resultChan := make(chan deleteResult, len(filePaths))

	// Create a context that can be cancelled if any critical error occurs
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go s.deleteWorker(workerCtx, jobChan, resultChan)
	}

	// Send jobs to workers
	go func() {
		defer close(jobChan)
		for _, filePath := range filePaths {
			select {
			case jobChan <- filePath:
			case <-workerCtx.Done():
				return // Context cancelled, stop sending jobs
			}
		}
	}()

	// Collect results
	response := &service.DeleteResponse{
		Deleted: make([]string, 0),
		Errors:  make([]service.DeleteError, 0),
	}

	for i := 0; i < len(filePaths); i++ {
		select {
		case result := <-resultChan:
			if result.Error != nil {
				errMsg := result.Error.Error()
				if errors.Is(result.Error, cloudstorage.ErrObjectNotExist) {
					errMsg = service.ErrNotFound.Error()
				}
				response.Errors = append(response.Errors, service.DeleteError{
					FilePath: result.FilePath,
					Error:    errMsg,
				})
			} else {
				response.Deleted = append(response.Deleted, result.FilePath)
			}
		case <-ctx.Done():
			// Context cancelled, return partial results
			return response, ctx.Err()
		}
	}

	return response, nil
}

// deleteResult represents the result of a file delete operation
type deleteResult struct {
	FilePath string
	Error    error
}

// deleteWorker is a worker goroutine that processes delete requests
func (s *Storage) deleteWorker(ctx context.Context, jobs <-chan string, results chan<- deleteResult) {
	bucket := s.client.GetBucket()
	
	for {
		select {
		case filePath, ok := <-jobs:
			if !ok {
				return // Channel closed, worker should exit
			}
			
			// Process the delete request
			err := s.deleteSingleFile(ctx, bucket, filePath)
			
			// Send result back
			select {
			case results <- deleteResult{
				FilePath: filePath,
				Error:    err,
			}:
			case <-ctx.Done():
				return // Context cancelled, worker should exit
			}
			
		case <-ctx.Done():
			return // Context cancelled, worker should exit
		}
	}
}

// deleteSingleFile performs the actual file delete operation
func (s *Storage) deleteSingleFile(ctx context.Context, bucket *cloudstorage.BucketHandle, filePath string) error {
	obj := bucket.Object(filePath)
	return obj.Delete(ctx)
}

func getExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}
