import { createStore } from "redux";
import { rulesReducer } from "../reducers/ruleReducer";

export const store = createStore(rulesReducer);