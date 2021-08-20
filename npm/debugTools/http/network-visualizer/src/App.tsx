import React from 'react';
import { useState } from 'react';
import { Stack, IStackTokens, IStackStyles } from '@fluentui/react';
import './App.css';
import { SrcDstGraph } from "./components/SrcDstGraph"
import { Pivot, PivotItem } from '@fluentui/react';
import { TextField } from '@fluentui/react/lib/TextField';
import { PrimaryButton } from '@fluentui/react/lib/Button';
import { RuleList } from './components/ruleList'
import { Separator } from '@fluentui/react/lib/Separator';
import { useDispatch } from "react-redux";
import { GetRules } from "./reducers/ruleReducer";


const stackTokens: IStackTokens = { childrenGap: 15 };
const stackStyles: Partial<IStackStyles> = {
  root: {
    marginTop: '100px',
    textAlign: 'start',
    color: '#605e5c',
  },
};
const innerStackStyles: Partial<IStackStyles> = {
  root: {
    textAlign: 'start',
  },
};
export const App: React.FunctionComponent = () => {
  const dispatch = useDispatch();
  const [src, setSrc] = useState<string>("");
  const [dst, setDst] = useState<string>("");
  const onChangeSrc = React.useCallback(
    (event: React.FormEvent<HTMLInputElement | HTMLTextAreaElement>, newValue?: string) => {
      setSrc(newValue || '');
    },
    [],
  );
  const onChangeDst = React.useCallback(
    (event: React.FormEvent<HTMLInputElement | HTMLTextAreaElement>, newValue?: string) => {
      setDst(newValue || '');
    },
    [],
  );

  const getRules = (source: string, destination: string) => {
    dispatch(GetRules(source, destination));
  }
  const onSubmit = () => {
    getRules(src, dst);
    setSrc("");
    setDst("");
  };

  return (

    <Stack horizontalAlign="center" verticalAlign="start" verticalFill styles={stackStyles} tokens={stackTokens}>
      <Pivot linkSize="large">

        <PivotItem headerText="Src-Dst Combinations">
          <Separator></Separator>

          <Stack horizontal verticalAlign="end" styles={innerStackStyles} tokens={stackTokens}>
            <TextField label="Source" value={src} onChange={onChangeSrc} />
            <TextField label="Destination" value={dst} onChange={onChangeDst} />
            <PrimaryButton text="Submit" allowDisabledFocus onClick={onSubmit} />

          </Stack>
          <SrcDstGraph></SrcDstGraph>
          <Separator></Separator>
          <RuleList></RuleList>

        </PivotItem>
      </Pivot>
    </Stack>
  );
};
