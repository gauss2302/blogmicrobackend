package clients

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"api-gateway/internal/config"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func buildClientTransportCredentials(tlsCfg config.GRPCTLSConfig) (credentials.TransportCredentials, error) {
	if !tlsCfg.Enabled {
		return insecure.NewCredentials(), nil
	}

	caPEM, err := os.ReadFile(tlsCfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read gRPC CA file: %w", err)
	}

	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("parse gRPC CA certs")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}

	if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
		clientCert, certErr := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
		if certErr != nil {
			return nil, fmt.Errorf("load gRPC client certificate: %w", certErr)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return credentials.NewTLS(tlsConfig), nil
}
