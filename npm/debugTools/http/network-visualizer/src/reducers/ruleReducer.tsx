import axios from 'axios';


export interface RulesState {
    rules: {
        ruleType: string,
        direction: string,
        srcIP: string,
        srcPort: string,
        dstIP: string,
        dstPort: string,
        protocol: string,
    }[];
}

const initialState = {
    rules: [],
};
axios.defaults.baseURL = 'localhost:10091/';

export type Action = { type: "GET_RULES"; src: string, dst: string };

export const GetRules = (source: string, destination: string): Action => ({
    type: "GET_RULES",
    src: source,
    dst: destination,
});

export const rulesReducer = (
    state: RulesState = initialState,
    action: Action
) => {
    switch (action.type) {
        case "GET_RULES": {
            axios.get('/getRules', {
                params: {
                    src: action.src,
                    dst: action.dst
                }
            })
                .then(function (response) {
                    // handle success
                    console.log(response.data);
                })
                .catch(function (error) {
                    // handle error
                    console.log(error);
                })


            return state;
        }
        default:
            return state;
    }
};
