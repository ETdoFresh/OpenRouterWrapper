const express = require('express');
const axios = require('axios');
const cors = require('cors');

const app = express();
const port = 5050;

// Middleware
app.use(cors());
app.use(express.json());

// OpenRouter API endpoint
const OPENROUTER_API_URL = 'https://openrouter.ai/api/v1';

// Chat completion endpoint
app.post('/v1/chat/completions', async (req, res) => {
    try {
        // Set response headers for streaming
        if (req.body.stream === true) {
            res.setHeader('Content-Type', 'text/event-stream');
            res.setHeader('Cache-Control', 'no-cache');
            res.setHeader('Connection', 'keep-alive');
        }

        const response = await axios.post(`${OPENROUTER_API_URL}/chat/completions`, req.body, {
            headers: {
                'Authorization': `${req.headers['authorization']}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                'X-Title': 'OpenRouter API Wrapper',
                'Content-Type': 'application/json'
            },
            responseType: req.body.stream === true ? 'stream' : 'json'
        });

        if (req.body.stream === true) {
            let retryCount = 0;
            const maxRetries = 5;
            const retryDelay = 2000; // 2 seconds

            const attemptStream = async () => {
                try {
                    const streamResponse = await axios.post(`${OPENROUTER_API_URL}/chat/completions`, req.body, {
                        headers: {
                            'Authorization': `${req.headers['authorization']}`,
                            'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                            'X-Title': 'OpenRouter API Wrapper',
                            'Content-Type': 'application/json'
                        },
                        responseType: 'stream'
                    });

                    let hasStarted = false;
                    const timeoutId = setTimeout(() => {
                        if (!hasStarted && retryCount < maxRetries) {
                            console.log(`Arrr! Stream didn't start in time, retry attempt ${retryCount + 1} of ${maxRetries}`);
                            retryCount++;
                            streamResponse.data.destroy();
                            attemptStream();
                        }
                    }, retryDelay);

                    streamResponse.data.on('data', (chunk) => {
                        hasStarted = true;
                        clearTimeout(timeoutId);
                        res.write(chunk);
                    });

                    streamResponse.data.on('end', () => {
                        clearTimeout(timeoutId);
                        res.end();
                    });

                    streamResponse.data.on('error', (error) => {
                        clearTimeout(timeoutId);
                        console.error('Stream error:', error);
                        if (retryCount < maxRetries) {
                            console.log(`Yarrr! Stream error, retry attempt ${retryCount + 1} of ${maxRetries}`);
                            retryCount++;
                            attemptStream();
                        } else {
                            console.error('All retry attempts failed!');
                            res.end();
                        }
                    });
                } catch (error) {
                    if (retryCount < maxRetries) {
                        console.log(`Blimey! Connection error, retry attempt ${retryCount + 1} of ${maxRetries}`);
                        retryCount++;
                        attemptStream();
                    } else {
                        console.error('All retry attempts failed!');
                        res.status(500).json({ error: 'Failed to establish stream after maximum retries' });
                    }
                }
            };

            await attemptStream();
        } else {
            res.json(response.data);
        }
    } catch (error) {
        console.error('Error:', error.response?.status, error.response?.data?.statusMessage);
        const errorResponse = {
            error: {
                message: error.response?.data?.error || error.response?.data?.statusMessage || error.message || 'Internal server error',
                status: error.response?.status || 500,
                type: error.name || 'Error'
            }
        };
        res.status(errorResponse.error.status).json(errorResponse);
    }
});

// Generation stats endpoint
app.get('/v1/generation', async (req, res) => {
    try {
        const response = await axios.get(`${OPENROUTER_API_URL}/generation?id=${req.query.id}`, {
            headers: {
                'Authorization': `${req.headers['authorization']}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                'X-Title': 'OpenRouter API Wrapper'
            }
        });
        res.json(response.data);
    } catch (error) {
        console.error('Error:', error.response?.status, error.response?.data?.statusMessage);
        const errorResponse = {
            error: {
                message: error.response?.data?.error || error.response?.data?.statusMessage || error.message || 'Internal server error',
                status: error.response?.status || 500,
                type: error.name || 'Error'
            }
        };
        res.status(errorResponse.error.status).json(errorResponse);
    }
});

// Models endpoint
app.get('/v1/models', async (req, res) => {
    try {
        const response = await axios.get(`${OPENROUTER_API_URL}/models`, {
            headers: {
                'Authorization': `${req.headers['authorization']}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:5050',
                'X-Title': 'OpenRouter API Wrapper'
            }
        });
        res.json(response.data);
    } catch (error) {
        console.error('Error:', error.response?.status, error.response?.data?.statusMessage);
        const errorResponse = {
            error: {
                message: error.response?.data?.error || error.response?.data?.statusMessage || error.message || 'Internal server error',
                status: error.response?.status || 500,
                type: error.name || 'Error'
            }
        };
        res.status(errorResponse.error.status).json(errorResponse);
    }
});

app.listen(port, () => {
    console.log(`🏴‍☠️ Server be sailin' on port ${port}! Arrr!`);
});
