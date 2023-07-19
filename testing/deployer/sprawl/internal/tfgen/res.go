package tfgen

import (
	"bytes"
	"text/template"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type FileResource struct {
	name string
	res  Resource
}

func (r *FileResource) Name() string { return r.name }

func (r *FileResource) Commit(logger hclog.Logger) error {
	val, err := r.res.Render()
	if err != nil {
		return err
	}
	_, err = UpdateFileIfDifferent(logger, []byte(val), r.name, 0644)
	return err
}

func File(name string, res Resource) *FileResource {
	return &FileResource{name: name, res: res}
}

func Text(s string) Resource {
	return &textResource{text: s}
}

func Embed(name string) Resource {
	return &embedResource{name: name}
}

func Eval(t *template.Template, data any) Resource {
	return &evalResource{template: t, data: data, hcl: false}
}

func HCL(t *template.Template, data any) Resource {
	return &evalResource{template: t, data: data, hcl: true}
}

type Resource interface {
	Render() (string, error)
}

type embedResource struct {
	name string
}

func (r *embedResource) Render() (string, error) {
	val, err := content.ReadFile(r.name)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

type textResource struct {
	text string
}

func (r *textResource) Render() (string, error) {
	return r.text, nil
}

type evalResource struct {
	template *template.Template
	data     any
	hcl      bool
}

func (r *evalResource) Render() (string, error) {
	out, err := StringTemplate(r.template, r.data)
	if err != nil {
		return "", err
	}

	if r.hcl {
		return string(hclwrite.Format([]byte(out))), nil
	}
	return out, nil
}

func StringTemplate(t *template.Template, data any) (string, error) {
	var res bytes.Buffer
	if err := t.Execute(&res, data); err != nil {
		return "", err
	}
	return res.String(), nil
}
