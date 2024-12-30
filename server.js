const express = require('express');
const axios = require('axios');
const cors = require('cors');
require('dotenv').config();

const app = express();
const port = process.env.PORT || 3000;

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
                'Authorization': `Bearer ${process.env.OPENROUTER_API_KEY}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:3000',
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
                            'Authorization': `Bearer ${process.env.OPENROUTER_API_KEY}`,
                            'HTTP-Referer': req.headers['referer'] || 'http://localhost:3000',
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
        console.error('Error:', error.response?.data || error.message);
        res.status(error.response?.status || 500).json({
            error: error.response?.data || 'Internal server error'
        });
    }
});

// Generation stats endpoint
app.get('/v1/generation/:id', async (req, res) => {
    try {
        const response = await axios.get(`${OPENROUTER_API_URL}/generation?id=${req.params.id}`, {
            headers: {
                'Authorization': `Bearer ${process.env.OPENROUTER_API_KEY}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:3000',
                'X-Title': 'OpenRouter API Wrapper'
            }
        });
        res.json(response.data);
    } catch (error) {
        console.error('Error:', error.response?.data || error.message);
        res.status(error.response?.status || 500).json({
            error: error.response?.data || 'Internal server error'
        });
    }
});

// Models endpoint
app.get('/v1/models', async (req, res) => {
    try {
        const response = await axios.get(`${OPENROUTER_API_URL}/models`, {
            headers: {
                'Authorization': `Bearer ${process.env.OPENROUTER_API_KEY}`,
                'HTTP-Referer': req.headers['referer'] || 'http://localhost:3000',
                'X-Title': 'OpenRouter API Wrapper'
            }
        });
        res.json(response.data);
    } catch (error) {
        console.error('Error:', error.response?.data || error.message);
        res.status(error.response?.status || 500).json({
            error: error.response?.data || 'Internal server error'
        });
    }
});

app.listen(port, () => {
    console.log(`🏴‍☠️ Server be sailin' on port ${port}! Arrr!`);
});
