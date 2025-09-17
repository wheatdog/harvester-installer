package widgets

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
)

type Option struct {
	Value string
	Text  string
}

type OptionState struct {
	option   Option
	selected bool
}

type GetOptionsFunc func() ([]Option, error)

type Select struct {
	*Panel
	Value           string
	getOptionsFunc  GetOptionsFunc
	multi           bool
	optionState     map[string]OptionState
	orderedValues   []string
	selectedValues  []string
}

func NewSelect(g *gocui.Gui, name string, text string, getOptionsFunc GetOptionsFunc) (*Select, error) {
	return &Select{
		Panel: &Panel{
			Name:    name,
			g:       g,
			Content: text,
		},
		getOptionsFunc: getOptionsFunc,
	}, nil
}

func (s *Select) Show() error {
	var err error
	if err := s.Panel.Show(); err != nil {
		return err
	}
	if err := s.updateOptions(); err != nil {
		return err
	}
	optionViewName := s.Name + "-options"
	offset := 0
	if len(s.Content) > 0 {
		offset = len(strings.Split(s.Content, "\n")) + 1
	}
	y0 := s.Y0 + offset
	y1 := s.Y0 + offset + len(s.orderedValues) + 1
	v, err := s.g.SetView(optionViewName, s.X0, y0, s.X1, y1)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Wrap = true

		if s.multi {
			if err = s.updateSelectedStatus(v); err != nil {
				return err
			}
		} else {
			v.Highlight = true
			v.SelBgColor = gocui.ColorGreen
			v.SelFgColor = gocui.ColorBlack

			foundOptIdx := -1
			for idx, value := range s.orderedValues {
				opt := s.optionState[value].option
				if _, err := fmt.Fprintln(v, opt.Text); err != nil {
					return err
				}
				if value == s.Value {
					foundOptIdx = idx
				}
			}

			// cursor should point to the current value if not empty
			if s.Value != "" {
				if foundOptIdx == -1 {
					return fmt.Errorf("'%s' not found in options", s.Value)
				}
				ox, oy := v.Origin()
				if err := v.SetCursor(ox, oy+foundOptIdx); err != nil {
					return err
				}
			}
		}

		if _, err := s.g.SetCurrentView(optionViewName); err != nil {
			return err
		}

		if err := s.setOptionsKeyBindings(optionViewName); err != nil {
			return err
		}
		if s.KeyBindings != nil {
			for key, f := range s.KeyBindings {
				if err := s.g.SetKeybinding(optionViewName, key, gocui.ModNone, f); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Select) Close() error {
	optionViewName := s.Name + "-options"
	s.g.DeleteKeybindings(optionViewName)
	if err := s.g.DeleteView(optionViewName); err != nil {
		return err
	}
	return s.Panel.Close()
}

func (s *Select) SetMulti(multi bool) {
	s.multi = multi

	if multi {
		if len(s.KeyBindingTips) == 0 {
			s.KeyBindingTips = map[string]string{}
		}
		s.KeyBindingTips["SPACE"] = "select options"
	} else {
		delete(s.KeyBindingTips, "SPACE")
	}
}

func (s *Select) GetData() (string, error) {
	optionViewName := s.Name + "-options"
	ov, err := s.g.View(optionViewName)
	if err != nil {
		return "", err
	}
	if len(ov.BufferLines()) == 0 {
		return "", nil
	}
	_, cy := ov.Cursor()
	var value string
	if len(s.orderedValues) >= cy+1 {
		value = s.orderedValues[cy]
	}
	return value, nil
}

func (s *Select) GetMultiData() []string {
	return s.selectedValues
}

func (s *Select) Reset() {
	s.Value = ""
	s.optionState = nil
	s.orderedValues = nil
	s.selectedValues = nil
}

func (s *Select) updateSelectedStatus(v *gocui.View) error {
	v.Clear()
	_, cy := v.Cursor()
	if err := v.SetCursor(1, cy); err != nil {
		return err
	}
	values := make([]string, 0)
	for _, value := range s.orderedValues {
		selected := " "
		state := s.optionState[value]
		if state.selected {
			selected = "x"
			values = append(values, value)
		}
		if _, err := fmt.Fprintf(v, "[%s] %s\n", selected, state.option.Text); err != nil {
			return err
		}
	}
	s.selectedValues = values
	s.Value = strings.Join(values, ",")
	return nil
}

func (s *Select) setOptionsKeyBindings(viewName string) error {
	if err := setOptionsKeyBindings(s.g, viewName); err != nil {
		return err
	}
	if s.multi {
		handler := func(_ *gocui.Gui, v *gocui.View) error {
			_, cy := v.Cursor()
			if len(s.orderedValues) >= cy+1 {
				value := s.orderedValues[cy]
				state := s.optionState[value]
				state.selected = !state.selected
				s.optionState[value] = state
			}
			return s.updateSelectedStatus(v)
		}
		if err := s.g.SetKeybinding(viewName, gocui.KeySpace, gocui.ModNone, handler); err != nil {
			return err
		}
	}
	return nil
}

func setOptionsKeyBindings(g *gocui.Gui, viewName string) error {
	if err := g.SetKeybinding(viewName, gocui.KeyArrowUp, gocui.ModNone, ArrowUp); err != nil {
		return err
	}
	if err := g.SetKeybinding(viewName, gocui.KeyArrowDown, gocui.ModNone, ArrowDown); err != nil {
		return err
	}
	return nil
}

func (s *Select) updateOptions() error {
	if s.getOptionsFunc == nil {
		return nil
	}

	options, err := s.getOptionsFunc();
	if err != nil {
		return err
	}

	s.orderedValues = make([]string, 0, len(options))
	newOptionState := make(map[string]OptionState, len(options))
	for _, opt := range(options) {
		s.orderedValues = append(s.orderedValues, opt.Value)
		state := s.optionState[opt.Value]
		state.option = opt
		newOptionState[opt.Value] = state
	}
	s.optionState = newOptionState

	newSelectedValues := make([]string, 0, len(s.selectedValues))
	for _, value := range(s.selectedValues) {
		if _, exists := s.optionState[value]; !exists {
			logrus.Warnf("value '%s' not found in options after updating", value)
			continue
		}
		newSelectedValues = append(newSelectedValues, value)
	}
	s.selectedValues = newSelectedValues

	if _, exists := s.optionState[s.Value]; s.Value != "" && !exists {
		logrus.Warnf("value '%s' not found in options after updating", s.Value)
		s.Value = ""
	}

	return nil
}

func (s *Select) pickOptionByValue(value string) (Option, bool) {
	state, exists := s.optionState[value]
	return state.option, exists
}

func (s *Select) pickOptionByIndex(idx int) (Option, bool) {
	if idx < 0 || idx >= len(s.orderedValues) {
		return Option{}, false
	}
	value := s.orderedValues[idx]
	return s.pickOptionByValue(value)
}

func (s *Select) getOptionCount() int {
	return len(s.orderedValues)
}
