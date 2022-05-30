package server

import (
	"path"

	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/server/handler/extenders"
)

const (
	apiPrefix = "/scheduler"
)

func (s *apiServer) initRouter() {
	s.installHandler()
}

func (s *apiServer) installHandler() {

	v1 := s.Group(apiPrefix)
	{
		predicateHandler := extenders.Predicate{
			Name: known.PredicateSafeOverloadName,
			Func: s.extenderScheduler.GetPredicatesFunc(known.PredicateSafeOverloadName),
		}
		priorityHandler := extenders.Prioritize{
			Name: known.PrioritySafeBalanceName,
			Func: s.extenderScheduler.GetPrioritiesFunc(known.PrioritySafeBalanceName),
		}

		v1.POST(path.Join("predicates", predicateHandler.Name), predicateHandler.Handler)
		v1.POST(path.Join("priorities", priorityHandler.Name), priorityHandler.Handler)
	}

}
