package genetics

import (
	"io"
	"bufio"
	"fmt"
	"gopkg.in/yaml.v2"
	"github.com/spf13/cast"
	"github.com/dmarkham/goNEAT/neat"
	"github.com/dmarkham/goNEAT/neat/network"
	"github.com/dmarkham/goNEAT/neat/utils"
)

// The interface to define genome writer
type GenomeWriter interface {
	// Writes Genome record
	WriteGenome(genome *Genome) error
}

// Creates genome writer with specified data encoding format
func NewGenomeWriter(w io.Writer, encoding GenomeEncoding) (GenomeWriter, error) {
	switch encoding {
	case PlainGenomeEncoding:
		return &plainGenomeWriter{w:bufio.NewWriter(w)}, nil
	case YAMLGenomeEncoding:
		return &yamlGenomeWriter{w:bufio.NewWriter(w)}, nil
	default:
		return nil, ErrUnsupportedGenomeEncoding
	}
}

// The plain text encoded genome writer
type plainGenomeWriter struct {
	w *bufio.Writer
}

// Writes genome in Plain Text format
func (wr *plainGenomeWriter) WriteGenome(g *Genome) error {
	_, err := fmt.Fprintf(wr.w, "genomestart %d\n", g.Id)
	if err != nil {
		return err
	}

	for _, tr := range g.Traits {
		fmt.Fprint(wr.w, "trait ")
		err := wr.writeTrait(tr)
		if err != nil {
			return err
		}
		fmt.Fprintln(wr.w, "")
	}

	for _, nd := range g.Nodes {
		fmt.Fprint(wr.w, "node ")
		err := wr.writeNetworkNode(nd)
		if err != nil {
			return err
		}
		fmt.Fprintln(wr.w, "")
	}

	for _, gn := range g.Genes {
		fmt.Fprint(wr.w, "gene ")
		err := wr.writeConnectionGene(gn)
		if err != nil {
			return err
		}
		fmt.Fprintln(wr.w, "")
	}
	_, err = fmt.Fprintf(wr.w, "genomeend %d\n", g.Id)

	// flush buffer
	err = wr.w.Flush()

	return err
}

// Dump trait in plain text format
func (wr *plainGenomeWriter) writeTrait(t *neat.Trait) error {
	_, err := fmt.Fprintf(wr.w, "%d ", t.Id)
	if err == nil {
		for i, p := range t.Params {
			if i < len(t.Params) - 1 {
				_, err = fmt.Fprintf(wr.w, "%g ", p)
			} else {
				_, err = fmt.Fprintf(wr.w, "%g", p)
			}

			if err != nil {
				return err
			}
		}
	}
	return err
}
// Dump network node in plain text format
func (wr *plainGenomeWriter) writeNetworkNode(n *network.NNode) error {
	trait_id := 0
	if n.Trait != nil {
		trait_id = n.Trait.Id
	}
	act_str, err := utils.NodeActivators.ActivationNameFromType(n.ActivationType)
	if err == nil {
		_, err = fmt.Fprintf(wr.w, "%d %d %d %d %s", n.Id, trait_id, n.NodeType(),
			n.NeuronType, act_str)
	}
	return err
}
// Dump connection gene in plain text format
func (wr *plainGenomeWriter) writeConnectionGene(g *Gene) error {
	link := g.Link
	traitId := 0
	if link.Trait != nil {
		traitId = link.Trait.Id
	}
	inNodeId := link.InNode.Id
	outNodeId := link.OutNode.Id
	weight := link.Weight
	recurrent := link.IsRecurrent
	innov_num := g.InnovationNum
	mut_num := g.MutationNum
	enabled := g.IsEnabled

	_, err := fmt.Fprintf(wr.w, "%d %d %d %g %t %d %g %t",
		traitId, inNodeId, outNodeId, weight, recurrent, innov_num, mut_num, enabled)
	return err
}

// The YAML encoded genome writer
type yamlGenomeWriter struct {
	w *bufio.Writer
}

func (wr *yamlGenomeWriter) WriteGenome(g *Genome) (err error) {
	g_map := make(map[string]interface{})
	g_map["id"] = g.Id

	// encode traits
	traits := make([]map[string]interface{}, len(g.Traits))
	for i, t := range g.Traits {
		traits[i] = wr.encodeGenomeTrait(t)
	}
	g_map["traits"] = traits

	// encode network nodes
	nodes := make([]map[string]interface{}, len(g.Nodes))
	for i, n := range g.Nodes {
		nodes[i], err = wr.encodeNetworkNode(n)
		if err != nil {
			return err
		}
	}
	g_map["nodes"] = nodes

	// encode connection genes
	genes := make([]map[string]interface{}, len(g.Genes))
	for i, gn := range g.Genes {
		genes[i] = wr.encodeConnectionGene(gn)
	}
	g_map["genes"] = genes

	// encode control genes if any
	if len(g.ControlGenes) > 0 {
		modules := make([]map[string]interface{}, len(g.ControlGenes))
		for i, cg := range g.ControlGenes {
			modules[i], err = wr.encodeControlGene(cg)
			if err != nil {
				return err
			}
		}
		g_map["modules"] = modules
	}


	// store genome map
	r_map := make(map[string]interface{})
	r_map["genome"] = g_map

	// encode everything as YAML
	enc := yaml.NewEncoder(wr.w)
	err = enc.Encode(r_map)
	if err == nil {
		// flush stream
		err = wr.w.Flush()
	}

	return err
}

func (wr *yamlGenomeWriter) encodeControlGene(gene *MIMOControlGene) (g_map map[string]interface{}, err error) {
	g_map = make(map[string]interface{})
	g_map["id"] = gene.ControlNode.Id
	if gene.ControlNode.Trait != nil {
		g_map["trait_id"] = gene.ControlNode.Trait.Id
	} else {
		g_map["trait_id"] = 0
	}
	g_map["innov_num"] = gene.InnovationNum
	g_map["mut_num"] = gene.MutationNum
	g_map["enabled"] = gene.IsEnabled
	g_map["activation"], err = utils.NodeActivators.ActivationNameFromType(gene.ControlNode.ActivationType)
	if err != nil {
		return nil, err
	}
	// store inputs
	inputs := make([]map[string]interface{}, len(gene.ControlNode.Incoming))
	for i, in := range gene.ControlNode.Incoming {
		inputs[i] = wr.encodeModuleLink(in.InNode.Id, i)
	}
	g_map["inputs"] = inputs

	// store outputs
	outputs := make([]map[string]interface{}, len(gene.ControlNode.Outgoing))
	for i, out := range gene.ControlNode.Outgoing {
		outputs[i] = wr.encodeModuleLink(out.OutNode.Id, i)
	}
	g_map["outputs"] = outputs

	return g_map, err
}

func (wr *yamlGenomeWriter) encodeModuleLink(id, order int) map[string]interface{} {
	l_map := make(map[string]interface{})
	l_map["id"] = id
	l_map["order"] = order
	return l_map
}

func (wr *yamlGenomeWriter) encodeConnectionGene(gene *Gene) map[string]interface{} {
	g_map := make(map[string]interface{})
	if gene.Link.Trait != nil {
		g_map["trait_id"] = gene.Link.Trait.Id
	} else {
		g_map["trait_id"] = 0
	}
	g_map["src_id"] = gene.Link.InNode.Id
	g_map["tgt_id"] = gene.Link.OutNode.Id
	g_map["innov_num"] = gene.InnovationNum
	g_map["weight"] = gene.Link.Weight
	g_map["mut_num"] = gene.MutationNum
	g_map["recurrent"] = cast.ToString(gene.Link.IsRecurrent)
	g_map["enabled"] = cast.ToString(gene.IsEnabled)
	return g_map
}

func (wr *yamlGenomeWriter) encodeNetworkNode(node *network.NNode) (n_map map[string]interface{}, err error) {
	n_map = make(map[string]interface{})
	n_map["id"] = node.Id
	if node.Trait != nil {
		n_map["trait_id"] = node.Trait.Id
	} else {
		n_map["trait_id"] = 0
	}
	n_map["type"] = network.NeuronTypeName(node.NeuronType)
	n_map["activation"], err = utils.NodeActivators.ActivationNameFromType(node.ActivationType)
	return n_map, err
}

func (wr *yamlGenomeWriter) encodeGenomeTrait(trait *neat.Trait) map[string]interface{} {
	tr_map := make(map[string]interface{})
	tr_map["id"] = trait.Id
	tr_map["params"] = trait.Params
	return tr_map
}