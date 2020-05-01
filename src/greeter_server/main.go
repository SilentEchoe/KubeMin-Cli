package main

import (
	"context"
	pb "github.com/AnAnonymousFriend/LearningNotes-Go/src/first"
	"sync"
)

const port  = ":50051"

type server struct {

}

type routeGuideServer struct {
	pb.UnimplementedRouteGuideServer
	savedFeatures []*pb.Feature // read-only after initialized

	mu         sync.Mutex // protects routeNotes
	routeNotes map[string][]*pb.RouteNote
}

func (s *server) GetFeature(ctx context.Context, point *pb.Point) (*pb.Feature, error) {

}

