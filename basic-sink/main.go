// based on https://github.com/salrashid123/envoy_ext_proc/blob/eca3b3a89929bf8cb80879ba553798ecea1c5622/grpc_server.go

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	service_ext_proc_v3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

var (
	grpcport = flag.String("grpcport", ":18080", "grpcport")
)

type server struct{}

type healthServer struct{}

func (s *healthServer) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	log.Printf("Handling grpc Check request: + %s", in.String())
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(in *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watch is not implemented")
}

func (s *server) Process(srv service_ext_proc_v3.ExternalProcessor_ProcessServer) error {
	log.Printf("Got stream:  -->  ")
	ctx := srv.Context()
	for {
		select {
		case <-ctx.Done():
			log.Printf("context done")
			return ctx.Err()
		default:
		}

		req, err := srv.Recv()
		if err == io.EOF {
			// envoy has closed the stream. Don't return anything and close this stream entirely
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "cannot receive stream request: %v", err)
		}

		// build response based on request type
		resp := &service_ext_proc_v3.ProcessingResponse{}
		switch v := req.Request.(type) {
		case *service_ext_proc_v3.ProcessingRequest_RequestHeaders:
			log.Printf("service_ext_proc_v3.ProcessingRequest_RequestHeaders :  %v \n", v)

			r := req.Request
			h := r.(*service_ext_proc_v3.ProcessingRequest_RequestHeaders)
			//log.Printf("Got RequestHeaders.Attributes %v", h.RequestHeaders.Attributes)
			//log.Printf("Got RequestHeaders.Headers %v", h.RequestHeaders.Headers)

			log.Printf("   Request: %+v\n", r)
			log.Printf("   Headers: %+v\n", h)
			for _, n := range h.RequestHeaders.Headers.Headers {
				log.Printf("      Header %s %s", n.Key, fmt.Sprintf("%s", n.RawValue))
			}
			resp = &service_ext_proc_v3.ProcessingResponse{
				Response: &service_ext_proc_v3.ProcessingResponse_RequestHeaders{},
			}

		case *service_ext_proc_v3.ProcessingRequest_RequestBody:
			log.Printf("service_ext_proc_v3.ProcessingRequest_RequestBody :  %v \n", v)
			r := req.Request
			b := r.(*service_ext_proc_v3.ProcessingRequest_RequestBody)
			log.Printf("   Request: %+v\n", r)
			log.Printf("   RequestBody: %s\n", string(b.RequestBody.Body))
			log.Printf("   EndOfStream: %T\n", b.RequestBody.EndOfStream)
			resp = &service_ext_proc_v3.ProcessingResponse{
				Response: &service_ext_proc_v3.ProcessingResponse_RequestBody{},
			}

		case *service_ext_proc_v3.ProcessingRequest_RequestTrailers:
			log.Printf("service_ext_proc_v3.ProcessingRequest_RequestTrailers (not currently handled)")

		case *service_ext_proc_v3.ProcessingRequest_ResponseHeaders:
			log.Printf("service_ext_proc_v3.ProcessingRequest_ResponseHeaders :  %v \n", v)

			r := req.Request
			h := r.(*service_ext_proc_v3.ProcessingRequest_ResponseHeaders)

			log.Printf("   Request: %+v\n", r)
			log.Printf("   Headers: %+v\n", h)
			for _, n := range h.ResponseHeaders.Headers.Headers {
				log.Printf("      Header %s %s", n.Key, fmt.Sprintf("%s", n.RawValue))
			}

			resp = &service_ext_proc_v3.ProcessingResponse{
				Response: &service_ext_proc_v3.ProcessingResponse_ResponseHeaders{},
			}

		case *service_ext_proc_v3.ProcessingRequest_ResponseBody:
			log.Printf("service_ext_proc_v3.ProcessingRequest_ResponseBody :  %v \n", v)

			r := req.Request
			b := r.(*service_ext_proc_v3.ProcessingRequest_ResponseBody)
			log.Printf("   Request: %+v\n", r)
			log.Printf("   ResponseBody: %s", string(b.ResponseBody.Body))
			log.Printf("   EndOfStream: %T\n", b.ResponseBody.EndOfStream)
			resp = &service_ext_proc_v3.ProcessingResponse{
				Response: &service_ext_proc_v3.ProcessingResponse_ResponseBody{},
			}

		case *service_ext_proc_v3.ProcessingRequest_ResponseTrailers:
			log.Printf("service_ext_proc_v3.ProcessingRequest_ResponseTrailers (not currently handled)")

		default:
			log.Printf("Unknown Request type %v", v)
		}

		// At this point we believe we have created a valid response...
		// note that this is sometimes not the case
		// anyways for now just send it
		log.Printf("Sending ProcessingResponse")
		if err := srv.Send(resp); err != nil {
			log.Printf("send error %v", err)
			return err
		}

	}
}

func main() {

	flag.Parse()

	lis, err := net.Listen("tcp", *grpcport)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(1000)}
	s := grpc.NewServer(sopts...)

	service_ext_proc_v3.RegisterExternalProcessorServer(s, &server{})

	grpc_health_v1.RegisterHealthServer(s, &healthServer{})

	log.Printf("Starting gRPC server on port %s", *grpcport)

	var gracefulStop = make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Printf("caught sig: %+v", sig)
		time.Sleep(time.Second)
		log.Printf("Graceful stop completed")
		os.Exit(0)
	}()
	err = s.Serve(lis)
	if err != nil {
		log.Fatalf("killing server with %v", err)
	}
}
