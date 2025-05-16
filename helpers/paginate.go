package helpers

import (
	"fmt"

	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoPaginate struct {
	limit int64
	page  int64
	sort  map[string]interface{}
}

func NewMongoPaginate(limit, page int64, sort map[string]interface{}) *mongoPaginate {
	if sort == nil {
		sort = make(map[string]interface{})
	}
	fmt.Println("Limit: ", limit, "Page: ", page, "Sort: ", sort)

	return &mongoPaginate{
		limit: limit,
		page:  page,
		sort:  sort,
	}
}
func (mp *mongoPaginate) GetPaginatedOpts() *mongoPaginate {
	return mp
}

func (mp *mongoPaginate) SortQuery(sort map[string]interface{}) *mongoPaginate {
	if sort != nil {
		mp.sort = sort
	}
	return mp
}

func (mp *mongoPaginate) BuildFindOptions() *options.FindOptions {

	fmt.Println("limt: ", mp.limit, "page: ", mp.page, "sort: ", mp.sort)

	return &options.FindOptions{
		Limit: &mp.limit,
		Skip:  &mp.page,
		Sort:  mp.sort,
	}
}
